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
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArraySet;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;
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

  /** Used to extract the ipAddress (IPv4 or IPv6) and port from server listener names. */
  private static final Pattern SERVER_LISTENER_NAME_PATTERN;

  /** Used to look for server listeners when creating new listeners streams. */
  private static final String SERVER_LISTENER_NAME_PREFIX;

  static {
    SERVER_LISTENER_NAME_PREFIX =
        Strings.split(XdsSnapshotBuilder.SERVER_LISTENER_RESOURCE_NAME_TEMPLATE, '=')[0] + "=";
    SERVER_LISTENER_NAME_PATTERN =
        Pattern.compile(Pattern.quote(SERVER_LISTENER_NAME_PREFIX) + "([0-9a-f:.]*):([0-9]{1,5})");
  }

  /** The actual cache. */
  private final SimpleCache<T> delegate;

  /** Hash function used to determine node hash from node proto. */
  private final NodeGroup<T> nodeHashFn;

  /** Tracks server listener addresses by node hash. */
  private final ConcurrentHashMap<T, Set<EndpointAddress>> serverListenerAddresses;

  private final ReadWriteLock serverListenerAddressesLock;

  /**
   * Create an xDS resource cache for the provided node hash function (<code>NodeGroup</code>).
   *
   * @param nodeHashFn function that returns a consistent identifier for a node.
   */
  public XdsSnapshotCache(@NotNull NodeGroup<T> nodeHashFn) {
    this.delegate = new SimpleCache<>(nodeHashFn);
    this.nodeHashFn = nodeHashFn;
    this.serverListenerAddresses = new ConcurrentHashMap<>();
    this.serverListenerAddressesLock = new ReentrantReadWriteLock();
  }

  /**
   * For each node hash in the cache, create a new snapshot based on the provided snapshot, with the
   * addition of server listeners per group.
   */
  public void setSnapshot(@NotNull XdsSnapshotBuilder snapshotBuilder) {
    delegate
        .groups()
        .forEach(
            nodeHash -> {
              if (serverListenerAddressesLock.readLock().tryLock()) {
                try {
                  Snapshot snapshotForNodeHash =
                      snapshotBuilder
                          .addServerListenerAddresses(serverListenerAddresses.get(nodeHash))
                          .build();
                  delegate.setSnapshot(nodeHash, snapshotForNodeHash);
                } finally {
                  serverListenerAddressesLock.readLock().unlock();
                }
              }
            });
  }

  /**
   * CreateWatch override that intercepts stream creation, and if it is a new Listener stream, does
   * the following:
   *
   * <ol>
   *   <li>Extracts addresses and ports of any server listeners in the request and adds them to the
   *       set of known server socket addresses for the node hash.
   *   <li>If there is no existing snapshot for creates a snapshot with the server listener and
   *       associated route configuration
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
    if (request != null
        && request.v3Request() != null
        && request.getResourceType() == ResourceType.LISTENER) {
      T nodeHash = nodeHashFn.hash(request.v3Request().getNode());
      if (serverListenerAddressesLock.writeLock().tryLock()) {
        try {
          Set<EndpointAddress> addressesBefore =
              serverListenerAddresses.computeIfAbsent(nodeHash, g -> new CopyOnWriteArraySet<>());
          Set<EndpointAddress> newAddresses =
              getServerListenerAddresses(request.getResourceNamesList());
          serverListenerAddresses.get(nodeHash).addAll(newAddresses);
          Set<EndpointAddress> addressesAfter = serverListenerAddresses.get(nodeHash);
          Snapshot oldSnapshot = delegate.getSnapshot(nodeHash);
          if (oldSnapshot == null || addressesAfter.size() > addressesBefore.size()) {
            Snapshot newSnapshot =
                new XdsSnapshotBuilder()
                    .addSnapshot(oldSnapshot)
                    .addServerListenerAddresses(addressesAfter)
                    .build();
            LOG.info(
                "Creating a new snapshot with serverListenerAddresses={} for nodeHash={}",
                addressesAfter,
                nodeHash);
            delegate.setSnapshot(nodeHash, newSnapshot);
          }
        } finally {
          serverListenerAddressesLock.writeLock().unlock();
        }
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

  @NotNull
  private Set<EndpointAddress> getServerListenerAddresses(
      @Nullable ProtocolStringList resourceNames) {
    if (resourceNames == null) {
      return Collections.emptySet();
    }
    return resourceNames.stream()
        .filter(name -> name != null && name.startsWith(SERVER_LISTENER_NAME_PREFIX))
        .flatMap(
            name -> {
              Matcher matcher = SERVER_LISTENER_NAME_PATTERN.matcher(name);
              if (!matcher.matches()) {
                return null;
              }
              return Stream.of(
                  new EndpointAddress(matcher.group(1), Integer.parseInt(matcher.group(2))));
            })
        .collect(Collectors.toUnmodifiableSet());
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
