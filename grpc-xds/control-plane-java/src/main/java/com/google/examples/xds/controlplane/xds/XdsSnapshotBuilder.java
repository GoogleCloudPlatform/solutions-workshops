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

import com.google.common.collect.ImmutableList;
import com.google.examples.xds.controlplane.applications.Application;
import com.google.examples.xds.controlplane.applications.ApplicationEndpoint;
import com.google.examples.xds.controlplane.xds.cds.ClusterBuilder;
import com.google.examples.xds.controlplane.xds.eds.ClusterLoadAssignmentBuilder;
import com.google.examples.xds.controlplane.xds.eds.LocalityPriorityMapper;
import com.google.examples.xds.controlplane.xds.lds.ApiListenerBuilder;
import com.google.examples.xds.controlplane.xds.lds.EnvoyListenerBuilder;
import com.google.examples.xds.controlplane.xds.lds.GrpcServerListenerBuilder;
import com.google.examples.xds.controlplane.xds.rds.RouteConfigForApiListenerBuilder;
import com.google.examples.xds.controlplane.xds.rds.RouteConfigForEnvoyBuilder;
import com.google.examples.xds.controlplane.xds.rds.RouteConfigForGrpcServerBuilder;
import io.envoyproxy.controlplane.cache.v3.Snapshot;
import io.envoyproxy.envoy.config.cluster.v3.Cluster;
import io.envoyproxy.envoy.config.endpoint.v3.ClusterLoadAssignment;
import io.envoyproxy.envoy.config.listener.v3.Listener;
import io.envoyproxy.envoy.config.route.v3.RouteConfiguration;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.Secret;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Builds xDS resource snapshots for the cache. */
public class XdsSnapshotBuilder<T> {

  private static final Logger LOG = LoggerFactory.getLogger(XdsSnapshotBuilder.class);

  private final Map<String, Listener> listeners = new HashMap<>();
  private final Map<String, RouteConfiguration> routeConfigurations = new HashMap<>();
  private final Map<String, Cluster> clusters = new HashMap<>();
  private final Map<String, ClusterLoadAssignment> clusterLoadAssignments = new HashMap<>();

  /** Enable merging of endpoints for applications from multiple EndpointSlices. */
  private final Map<String, Set<ApplicationEndpoint>> endpointsByCluster = new HashMap<>();

  /** Addresses for server listeners to be added to the snapshot. */
  private final Set<EndpointAddress> grpcServerListenerAddresses = new HashSet<>();

  private final T nodeHash;
  private final LocalityPriorityMapper<T> localityPriorityMapper;
  private final XdsFeatures xdsFeatures;
  private final String authority;

  /** Creates a builder for an xDS resource cache snapshot. */
  public XdsSnapshotBuilder(
      @NotNull T nodeHash,
      @NotNull LocalityPriorityMapper<T> localityPriorityMapper,
      @NotNull XdsFeatures xdsFeatures,
      @NotNull String authority) {
    this.nodeHash = nodeHash;
    this.localityPriorityMapper = localityPriorityMapper;
    this.xdsFeatures = xdsFeatures;
    this.authority = authority;
  }

  /**
   * Add the provided application configurations to the xDS resource snapshot.
   *
   * @param apps configuration for gRPC applications
   */
  @SuppressWarnings("UnusedReturnValue")
  @NotNull
  public XdsSnapshotBuilder<T> addGrpcApplications(@NotNull Set<Application> apps) {
    for (Application app : apps) {
      if (!listeners.containsKey(app.name())) {
        var listener = new ApiListenerBuilder(app.name()).build();
        listeners.put(listener.getName(), listener);
        if (xdsFeatures.enableFederation()) {
          var xdstpListener =
              new ApiListenerBuilder(xdstpListener(authority, app.name()))
                  .withStatPrefix(app.name())
                  .withRouteConfigurationName(xdstpRouteConfiguration(authority, app.name()))
                  .build();
          listeners.put(xdstpListener.getName(), xdstpListener);
        }
      }
      if (!routeConfigurations.containsKey(app.name())) {
        var routeConfiguration = new RouteConfigForApiListenerBuilder(app.name()).build();
        routeConfigurations.put(routeConfiguration.getName(), routeConfiguration);
        if (xdsFeatures.enableFederation()) {
          var xdstpRouteConfiguration =
              new RouteConfigForApiListenerBuilder(xdstpRouteConfiguration(authority, app.name()))
                  .withVirtualHostName(app.name())
                  .withClusterName(xdstpCluster(authority, app.name()))
                  .build();
          routeConfigurations.put(xdstpRouteConfiguration.getName(), xdstpRouteConfiguration);
        }
      }
      if (!clusters.containsKey(app.name())) {
        var cluster =
            new ClusterBuilder(app.name())
                .withConnectTimeoutSeconds(3)
                .withHealthCheck(app.healthCheckProtocol(), app.healthCheckPort(), "")
                .withEnableTls(xdsFeatures.enableDataPlaneTls())
                .withServerAuthorization("[^/]+", app.namespace(), app.serviceAccountName())
                .withRequireClientCerts(xdsFeatures.requireDataPlaneClientCerts())
                .build();
        clusters.put(cluster.getName(), cluster);
        if (xdsFeatures.enableFederation()) {
          var xdstpCluster =
              new ClusterBuilder(xdstpCluster(authority, app.name()))
                  .withEdsServiceName(xdstpEdsService(authority, app.name()))
                  .withConnectTimeoutSeconds(5)
                  .withHealthCheck(app.healthCheckProtocol(), app.healthCheckPort(), "")
                  .withEnableTls(xdsFeatures.enableDataPlaneTls())
                  .withServerAuthorization("[^/]+", app.namespace(), app.serviceAccountName())
                  .withRequireClientCerts(xdsFeatures.requireDataPlaneClientCerts())
                  .build();
          clusters.put(xdstpCluster.getName(), xdstpCluster);
        }
      }
      var endpointsByClusterKey = app.name() + "-" + app.servingPort();
      if (endpointsByCluster.containsKey(endpointsByClusterKey)
          && !endpointsByCluster.get(endpointsByClusterKey).isEmpty()) {
        LOG.info(
            "Merging endpoints for app={} existingEndpoints=[{}], newEndpoints[{}]",
            app.name(),
            endpointsByCluster.get(endpointsByClusterKey),
            app.endpoints());
      }
      endpointsByCluster
          .computeIfAbsent(endpointsByClusterKey, key -> new HashSet<>())
          .addAll(app.endpoints());
      var clusterLoadAssignment =
          new ClusterLoadAssignmentBuilder<T>(
                  endpointsByCluster.get(endpointsByClusterKey), app.servingPort())
              .withEdsServiceName(app.name())
              .withLocalityPriorityMapper(nodeHash, localityPriorityMapper)
              .build();
      clusterLoadAssignments.put(clusterLoadAssignment.getClusterName(), clusterLoadAssignment);
      if (xdsFeatures.enableFederation()) {
        var xdstpClusterLoadAssignment =
            new ClusterLoadAssignmentBuilder<T>(
                    endpointsByCluster.get(endpointsByClusterKey), app.servingPort())
                .withEdsServiceName(xdstpEdsService(authority, app.name()))
                .withLocalityPriorityMapper(nodeHash, localityPriorityMapper)
                .build();
        clusterLoadAssignments.put(
            xdstpClusterLoadAssignment.getClusterName(), xdstpClusterLoadAssignment);
      }
    }
    return this;
  }

  @NotNull
  static String xdstpListener(@NotNull String authority, @NotNull String listenerName) {
    return "xdstp://%s/envoy.config.listener.v3.Listener/%s".formatted(authority, listenerName);
  }

  @NotNull
  static String xdstpRouteConfiguration(
      @NotNull String authority, @NotNull String routeConfigurationName) {
    return "xdstp://%s/envoy.config.route.v3.RouteConfiguration/%s"
        .formatted(authority, routeConfigurationName);
  }

  @NotNull
  static String xdstpCluster(@NotNull String authority, @NotNull String clusterName) {
    return "xdstp://%s/envoy.config.cluster.v3.Cluster/%s".formatted(authority, clusterName);
  }

  @NotNull
  static String xdstpEdsService(@NotNull String authority, @NotNull String serviceName) {
    return "xdstp://%s/envoy.config.endpoint.v3.ClusterLoadAssignment/%s"
        .formatted(authority, serviceName);
  }

  @NotNull
  @SuppressWarnings("UnusedReturnValue")
  public XdsSnapshotBuilder<T> addGrpcServerListenerAddresses(
      @NotNull Collection<EndpointAddress> addresses) {
    grpcServerListenerAddresses.addAll(addresses);
    return this;
  }

  /** Builds an xDS resource snapshot. */
  @NotNull
  public Snapshot build() {
    for (EndpointAddress address : grpcServerListenerAddresses) {
      var grpcServerListener =
          new GrpcServerListenerBuilder(address.ipAddress(), address.port())
              .withEnableTls(xdsFeatures.enableDataPlaneTls())
              .withRequireClientCerts(xdsFeatures.requireDataPlaneClientCerts())
              .withEnableRbac(xdsFeatures.enableRbac())
              .build();
      listeners.put(grpcServerListener.getName(), grpcServerListener);
    }
    if (!grpcServerListenerAddresses.isEmpty()) {
      var routeConfigForGrpcServerListener =
          new RouteConfigForGrpcServerBuilder(xdsFeatures.enableRbac()).build();
      routeConfigurations.put(
          routeConfigForGrpcServerListener.getName(), routeConfigForGrpcServerListener);
    }

    // Envoy proxies will not accept the gRPC server Listeners, because all the routes in their
    // RouteConfigurations specify `NonForwardingAction` as the action.
    // Envoy proxies will also not accept the API Listeners created for gRPC clients, because Envoy
    // proxies can only have at most one API Listener defined, and that API Listener must be a
    // static resource (not fetched via xDS).
    var envoyListener = new EnvoyListenerBuilder(50051).withEnableTls(true).build();
    listeners.put(envoyListener.getName(), envoyListener);
    var routeConfigForEnvoy = new RouteConfigForEnvoyBuilder(clusters.keySet()).build();
    routeConfigurations.put(routeConfigForEnvoy.getName(), routeConfigForEnvoy);

    // Not serving dynamic TLS certs via SDS. gRPC doesn't support SDS.
    // For Envoy proxies, rely on
    ImmutableList<Secret> secrets = ImmutableList.of();
    LOG.info(
        "Creating xDS resource snapshot with listeners={}",
        listeners.values().stream().map(Listener::getName).toList());
    String version = String.valueOf(System.nanoTime());
    return Snapshot.create(
        clusters.values(),
        clusterLoadAssignments.values(),
        listeners.values(),
        routeConfigurations.values(),
        secrets,
        version);
  }
}
