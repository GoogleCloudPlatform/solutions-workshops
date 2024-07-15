// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.examples.xds.controlplane.xds;

import com.google.examples.xds.controlplane.applications.Application;
import com.google.examples.xds.controlplane.applications.ApplicationCache;
import com.google.examples.xds.controlplane.xds.eds.LocalityPriorityMapper;
import com.google.protobuf.ProtocolStringList;
import io.envoyproxy.controlplane.cache.ConfigWatcher;
import io.envoyproxy.controlplane.cache.DeltaResponse;
import io.envoyproxy.controlplane.cache.DeltaWatch;
import io.envoyproxy.controlplane.cache.DeltaXdsRequest;
import io.envoyproxy.controlplane.cache.NodeGroup;
import io.envoyproxy.controlplane.cache.Resources.ResourceType;
import io.envoyproxy.controlplane.cache.Response;
import io.envoyproxy.controlplane.cache.Watch;
import io.envoyproxy.controlplane.cache.XdsRequest;
import io.envoyproxy.controlplane.cache.v3.SimpleCache;
import io.envoyproxy.controlplane.cache.v3.Snapshot;
import java.util.Collections;
import java.util.Map;
import java.util.Set;
import java.util.function.Consumer;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.bouncycastle.util.Strings;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * XdsSnapshotCache stores snapshots of xDS resources in a delegate cache.
 *
 * <p>It handles the initial server listener and route configuration request by intercepting
 * Listener stream creation, see createWatch().
 *
 * <p>It also handles propagating snapshots to all node hashes in the cache. This is currently done
 * naively, by using the same snapshot for all node hashes.
 *
 * @param <T> node hash type
 */
public class XdsSnapshotCache<T> implements ConfigWatcher {

  private static final Logger LOG = LoggerFactory.getLogger(XdsSnapshotCache.class);

  /**
   * Listener name template used by xDS-enabled gRPC servers.
   *
   * <p>Must match the value of <code>server_listener_resource_name_template</code> in the gRPC xDS
   * bootstrap configuration.
   *
   * <p>Using the template value from gRPC-Java unit tests, and the <a
   * href="https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/v0.15.0/main.go#L300">Traffic
   * Director gRPC bootstrap utility</a>, but this is not important.
   *
   * @see <a
   *     href="https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol">gRFC
   *     A36: xDS-Enabled Servers</a>
   */
  public static final String GRPC_SERVER_LISTENER_RESOURCE_NAME_TEMPLATE =
      "grpc/server?xds.resource.listening_address=%s";

  /**
   * Prefix used to look for gRPC server listeners when creating new Listener streams. gRPC server
   * Listener resource names use templates such as `grpc/server?xds.resource.listening_address=%s`.
   * SERVER_LISTENER_NAME_PREFIX is the part up to and including the `=` sign.
   */
  private static final String GRPC_SERVER_LISTENER_NAME_PREFIX;

  /**
   * Regex pattern used to extract the ipAddress (IPv4 or IPv6) and servingPort from gRPC server
   * Listener names.
   */
  private static final Pattern GRPC_SERVER_LISTENER_NAME_PATTERN;

  static {
    GRPC_SERVER_LISTENER_NAME_PREFIX =
        Strings.split(GRPC_SERVER_LISTENER_RESOURCE_NAME_TEMPLATE, '=')[0] + "=";

    GRPC_SERVER_LISTENER_NAME_PATTERN =
        Pattern.compile(
            Pattern.quote(GRPC_SERVER_LISTENER_NAME_PREFIX) + "([0-9a-f:.]*):([0-9]{1,5})");
  }

  /** The actual cache. */
  private final SimpleCache<T> delegate;

  /** Hash function used to determine node hash from node proto. */
  private final NodeGroup<T> nodeHashFn;

  /**
   * Constructs a priority map for localities, to be used in EDS ClusterLoadAssignment resources.
   */
  private final LocalityPriorityMapper<T> localityPriorityMapper;

  /** The authority name of this control plane for xDS federation. */
  private final String authority;

  /**
   * appsCache stores the most recent gRPC application configuration information from k8s cluster
   * EndpointSlices. The appsCache is used to populate new entries (previously unseen `nodeHash`es)
   * in the xDS resource snapshot cache, so that the new subscribers don't have to wait for an
   * EndpointSlice update before they can receive xDS resources.
   */
  private final ApplicationCache appsCache;

  /**
   * grpcServerListenerCache stores known server listener names for each snapshot cache key
   * (`nodeHash`). These names are captured when new Listener streams are created, see
   * `CreateWatch()`. The server listener names are added to xDS resource snapshots, to be included
   * in LDS responded for xDS-enabled gRPC servers.
   */
  private final GrpcServerListenerCache<T> grpcServerListenerCache;

  /** xdsFeatures contains flags to enable and disable xDS features, e.g., mTLS. */
  private final XdsFeatures xdsFeatures;

  /**
   * Create an xDS resource cache for the provided node hash function (<code>NodeGroup</code>).
   *
   * @param nodeHashFn function that returns a consistent identifier for a node.
   * @param authority the authority name of this control plane management server, must match the key
   *     in the gRPC xDS bootstrap config authorities section.
   */
  public XdsSnapshotCache(
      @NotNull NodeGroup<T> nodeHashFn,
      @NotNull LocalityPriorityMapper<T> localityPriorityMapper,
      @NotNull XdsFeatures xdsFeatures,
      @NotNull String authority) {
    this.delegate = new SimpleCache<>(nodeHashFn);
    this.nodeHashFn = nodeHashFn;
    this.localityPriorityMapper = localityPriorityMapper;
    this.xdsFeatures = xdsFeatures;
    this.authority = authority;
    this.appsCache = new ApplicationCache();
    this.grpcServerListenerCache = new GrpcServerListenerCache<>();
  }

  /**
   * CreateWatch override that intercepts stream creation, and if it is a new Listener stream, does
   * the following:
   *
   * <ol>
   *   <li>Extracts addresses and ports of any gRPC server Listeners in the request and adds them to
   *       the set of known server socket addresses for the snapshot (by node hash).
   *   <li>If there is no existing snapshot for the node hash in the request, creates a snapshot
   *       with the gRPC server Listener and associated RouteConfiguration.
   * </ol>
   *
   * <p>This solves (in a slightly hacky way) bootstrapping of xDS-enabled gRPC servers.
   */
  @Override
  public Watch createWatch(
      boolean ads,
      XdsRequest request,
      Set<String> knownResourceNames,
      Consumer<Response> responseConsumer,
      boolean hasClusterChanged,
      boolean allowDefaultEmptyEdsUpdate) {
    if (isListenerRequest(request)) {
      T nodeHash = nodeHashFn.hash(request.v3Request().getNode());
      Set<EndpointAddress> grpcServerListenerAddressesFromRequest =
          getGrpcServerListenerAddresses(request.getResourceNamesList());
      LOG.info("grpcServerListenerAddressesFromRequest={}", grpcServerListenerAddressesFromRequest);
      boolean isAdded =
          grpcServerListenerCache.add(nodeHash, grpcServerListenerAddressesFromRequest);
      if (isAdded || delegate.getSnapshot(nodeHash) == null) {
        var snapshot = createNewSnapshot(nodeHash, appsCache.getAll());
        delegate.setSnapshot(nodeHash, snapshot);
      }
    }
    return delegate.createWatch(
        ads,
        request,
        knownResourceNames,
        responseConsumer,
        hasClusterChanged,
        allowDefaultEmptyEdsUpdate);
  }

  private boolean isListenerRequest(@Nullable XdsRequest request) {
    return request != null
        && request.v3Request() != null
        && (request.v3Request().getResourceNamesCount() > 0
            || "envoy".equalsIgnoreCase(request.v3Request().getNode().getUserAgentName()))
        && request.getResourceType() == ResourceType.LISTENER;
  }

  /**
   * Looks for gRPC server Listener names in the provided resource names, and extracts the address
   * and servingPort for each gRPC server Listener found.
   *
   * <p>TODO: Handle xDS federation server Listener names using `xdstp://` names, e.g., {@code
   * xdstp://xds-authority.example.com/envoy.config.listener.v3.Listener/grpc/server/%s}.
   */
  @NotNull
  private Set<EndpointAddress> getGrpcServerListenerAddresses(
      @Nullable ProtocolStringList resourceNames) {
    if (resourceNames == null) {
      return Collections.emptySet();
    }
    return resourceNames.stream()
        .filter(name -> name != null && name.startsWith(GRPC_SERVER_LISTENER_NAME_PREFIX))
        .flatMap(
            name -> {
              Matcher matcher = GRPC_SERVER_LISTENER_NAME_PATTERN.matcher(name);
              if (!matcher.matches()) {
                return null;
              }
              return Stream.of(
                  new EndpointAddress(matcher.group(1), Integer.parseInt(matcher.group(2))));
            })
        .collect(Collectors.toUnmodifiableSet());
  }

  /**
   * UpdateResources creates a new snapshot for each node hash in the cache, based on the provided
   * gRPC application configuration, with the addition of gRPC server Listeners, an Envoy Listener,
   * and their associated RouteConfigurations.
   *
   * @param kubecontext the kubeconfig context where the gRPC applications came from
   * @param namespace the Kubernetes namespace of the informer
   * @param appsForContextAndNamespace gRPC application configuration to add to the new snapshot
   */
  public void updateResources(
      String kubecontext,
      @NotNull String namespace,
      @NotNull Set<Application> appsForContextAndNamespace) {
    boolean changesMade = appsCache.set(kubecontext, namespace, appsForContextAndNamespace);
    if (!changesMade) {
      LOG.info(
          "No changes to the application configuration, "
              + "so not creating new xDS resource snapshots.");
      return;
    }
    LOG.info("Creating new xDS resource snapshots for nodeHashes={}", delegate.groups());
    var apps = appsCache.getAll();
    delegate
        .groups()
        .forEach(
            nodeHash -> {
              LOG.info("Creating new xDS resource snapshot for nodeHash={}", nodeHash);
              var snapshot = createNewSnapshot(nodeHash, apps);
              delegate.setSnapshot(nodeHash, snapshot);
            });
  }

  @NotNull
  private Snapshot createNewSnapshot(T nodeHash, Set<Application> apps) {
    return new XdsSnapshotBuilder<>(nodeHash, localityPriorityMapper, xdsFeatures, authority)
        .addGrpcApplications(apps)
        .addGrpcServerListenerAddresses(grpcServerListenerCache.get(nodeHash))
        .build();
  }

  /** Just delegating, since delta xDS is not supported by this control plane implementation. */
  @Override
  public DeltaWatch createDeltaWatch(
      DeltaXdsRequest request,
      String requesterVersion,
      Map<String, String> resourceVersions,
      Set<String> pendingResources,
      boolean isWildcard,
      Consumer<DeltaResponse> responseConsumer,
      boolean hasClusterChanged) {
    return delegate.createDeltaWatch(
        request,
        requesterVersion,
        resourceVersions,
        pendingResources,
        isWildcard,
        responseConsumer,
        hasClusterChanged);
  }
}
