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
import com.google.protobuf.Any;
import com.google.protobuf.BoolValue;
import com.google.protobuf.UInt32Value;
import com.google.protobuf.util.Durations;
import io.envoyproxy.controlplane.cache.v3.Snapshot;
import io.envoyproxy.envoy.config.cluster.v3.Cluster;
import io.envoyproxy.envoy.config.cluster.v3.Cluster.DiscoveryType;
import io.envoyproxy.envoy.config.cluster.v3.Cluster.EdsClusterConfig;
import io.envoyproxy.envoy.config.core.v3.Address;
import io.envoyproxy.envoy.config.core.v3.AggregatedConfigSource;
import io.envoyproxy.envoy.config.core.v3.ApiVersion;
import io.envoyproxy.envoy.config.core.v3.ConfigSource;
import io.envoyproxy.envoy.config.core.v3.HealthStatus;
import io.envoyproxy.envoy.config.core.v3.Locality;
import io.envoyproxy.envoy.config.core.v3.SocketAddress;
import io.envoyproxy.envoy.config.core.v3.SocketAddress.Protocol;
import io.envoyproxy.envoy.config.core.v3.TrafficDirection;
import io.envoyproxy.envoy.config.endpoint.v3.ClusterLoadAssignment;
import io.envoyproxy.envoy.config.endpoint.v3.LbEndpoint;
import io.envoyproxy.envoy.config.endpoint.v3.LocalityLbEndpoints;
import io.envoyproxy.envoy.config.listener.v3.ApiListener;
import io.envoyproxy.envoy.config.listener.v3.Filter;
import io.envoyproxy.envoy.config.listener.v3.FilterChain;
import io.envoyproxy.envoy.config.listener.v3.Listener;
import io.envoyproxy.envoy.config.route.v3.Decorator;
import io.envoyproxy.envoy.config.route.v3.NonForwardingAction;
import io.envoyproxy.envoy.config.route.v3.Route;
import io.envoyproxy.envoy.config.route.v3.RouteAction;
import io.envoyproxy.envoy.config.route.v3.RouteConfiguration;
import io.envoyproxy.envoy.config.route.v3.RouteMatch;
import io.envoyproxy.envoy.config.route.v3.VirtualHost;
import io.envoyproxy.envoy.extensions.filters.http.fault.v3.HTTPFault;
import io.envoyproxy.envoy.extensions.filters.http.router.v3.Router;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager.CodecType;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager.ForwardClientCertDetails;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager.SetCurrentClientCertDetails;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager.UpgradeConfig;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpFilter;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.Rds;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.Secret;
import java.util.Collection;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

/** Builds xDS resource snapshots for the cache. */
public class XdsSnapshotBuilder {
  /** Copied from {@link io.envoyproxy.controlplane.cache.Resources}. */
  private static final String ENVOY_HTTP_CONNECTION_MANAGER = "envoy.http_connection_manager";

  /** Copied from {@link io.envoyproxy.controlplane.cache.Resources}. */
  private static final String ENVOY_FILTER_HTTP_ROUTER = "envoy.filters.http.router";

  private static final String ENVOY_FILTER_HTTP_FAULT = "envoy.filters.http.fault";

  /**
   * Listener name template used xDS clients that are gRPC servers.
   *
   * <p>Must match the value of <code>server_listener_resource_name_template</code> in the gRPC xDS
   * bootstrap configuration.
   *
   * <p>Using the template value from gRPC-Java unit tests, but this is not important.
   *
   * @see <a
   *     href="https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol">gRFC
   *     A36: xDS-Enabled Servers</a>
   */
  private static final String SERVER_LISTENER_RESOURCE_NAME_TEMPLATE =
      "grpc/server?xds.resource.listening_address=%s";

  /** The IPv4 listener address for xDS clients that serve gRPC services. */
  private static final String SERVER_LISTENER_ADDRESS_IPV4 = "0.0.0.0";

  /** The IPv6 listener address for xDS clients that serve gRPC services. */
  private static final String SERVER_LISTENER_ADDRESS_IPV6 = "[::]";

  /**
   * The listener port that will be pushed to the xDS clients for serving gRPC services. Must match
   * the local port used by the xDS client's server. TODO: Improve this so servers can listen on
   * other ports.
   */
  private static final int SERVER_LISTENER_PORT = 50051;

  /** Using the same route configuration name as Traffic Director, but this is not important. */
  private static final String SERVER_LISTENER_ROUTE_CONFIG_NAME =
      "inbound|default_inbound_config-50051";

  private final Map<String, Listener> listeners = new HashMap<>();
  private final Map<String, RouteConfiguration> routeConfigurations = new HashMap<>();
  private final Map<String, Cluster> clusters = new HashMap<>();
  private final Map<String, ClusterLoadAssignment> clusterLoadAssignments = new HashMap<>();

  /** Creates a builder for an xDS resource cache snapshot. */
  public XdsSnapshotBuilder() {
    // The server listeners and accompanying route configuration must always be present so that
    // xDS clients that serve gRPC services can bootstrap their server listeners.
    Listener serverListenerIpv4 =
        createServerListener(
            SERVER_LISTENER_ADDRESS_IPV4, SERVER_LISTENER_PORT, SERVER_LISTENER_ROUTE_CONFIG_NAME);
    listeners.put(serverListenerIpv4.getName(), serverListenerIpv4);
    Listener serverListenerIpv6 =
        createServerListener(
            SERVER_LISTENER_ADDRESS_IPV6, SERVER_LISTENER_PORT, SERVER_LISTENER_ROUTE_CONFIG_NAME);
    listeners.put(serverListenerIpv6.getName(), serverListenerIpv6);
    RouteConfiguration routeConfigForServerListener =
        createRouteConfigForServerListener(SERVER_LISTENER_ROUTE_CONFIG_NAME);
    routeConfigurations.put(routeConfigForServerListener.getName(), routeConfigForServerListener);
  }

  /**
   * Add the provided application configurations to the xDS resource snapshot.
   *
   * <p>TODO: There can be more than one EndpointSlice for a k8s Service. Check if there's already
   * an application with the same name and merge.
   *
   * @param apps configuration for gRPC applications
   */
  @SuppressWarnings("UnusedReturnValue")
  public XdsSnapshotBuilder addGrpcApplications(GrpcApplication... apps) {
    for (GrpcApplication app : apps) {
      Listener listener = createApiListener(app.listenerName(), app.routeName());
      listeners.put(listener.getName(), listener);
      RouteConfiguration routeConfiguration =
          createRouteConfiguration(
              app.routeName(), app.listenerName(), app.pathPrefix(), app.clusterName());
      routeConfigurations.put(routeConfiguration.getName(), routeConfiguration);
      Cluster cluster = createCluster(app.clusterName());
      clusters.put(cluster.getName(), cluster);
      ClusterLoadAssignment clusterLoadAssignment =
          createClusterLoadAssignment(app.clusterName(), app.port(), app.endpoints());
      clusterLoadAssignments.put(clusterLoadAssignment.getClusterName(), clusterLoadAssignment);
    }
    return this;
  }

  /** Builds an xDS resource snapshot. */
  public Snapshot build() {
    String version = String.valueOf(System.nanoTime());
    ImmutableList<Secret> secrets = ImmutableList.of();
    return Snapshot.create(
        clusters.values(),
        clusterLoadAssignments.values(),
        listeners.values(),
        routeConfigurations.values(),
        secrets,
        version);
  }

  /**
   * Application listener (<a
   * href="https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#listener-proto">RFC</a>).
   *
   * @param listenerName the name to use for the listener, also used for statPrefix.
   * @param routeName the name of the RDS route configuration for this API listener.
   * @return an LDS API listener for a gRPC application.
   */
  private Listener createApiListener(String listenerName, String routeName) {
    var httpConnectionManager =
        HttpConnectionManager.newBuilder()
            .setCodecType(CodecType.AUTO)
            // https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/stats#config-http-conn-man-stats
            .setStatPrefix(listenerName)
            .setRds(
                Rds.newBuilder()
                    .setConfigSource(
                        ConfigSource.newBuilder()
                            .setResourceApiVersion(ApiVersion.V3)
                            .setAds(AggregatedConfigSource.getDefaultInstance())
                            .build())
                    .setRouteConfigName(routeName)
                    .build())
            .addHttpFilters(
                // Enable fault injection.
                HttpFilter.newBuilder()
                    .setName(ENVOY_FILTER_HTTP_FAULT)
                    .setTypedConfig(Any.pack(HTTPFault.getDefaultInstance()))
                    .build())
            .addHttpFilters(
                // Router must be the last filter.
                HttpFilter.newBuilder()
                    .setName(ENVOY_FILTER_HTTP_ROUTER)
                    .setTypedConfig(
                        Any.pack(Router.newBuilder().setSuppressEnvoyHeaders(true).build()))
                    .build())
            .build();

    return Listener.newBuilder()
        .setName(listenerName)
        .setApiListener(
            ApiListener.newBuilder().setApiListener(Any.pack(httpConnectionManager)).build())
        .build();
  }

  /**
   * Create the server listener for xDS clients that are also gRPC servers.
   *
   * @return a Listener, using RDS for the RouteConfiguration
   */
  private Listener createServerListener(String address, int port, String routeConfigName) {
    var httpConnectionManager =
        HttpConnectionManager.newBuilder()
            .setCodecType(CodecType.AUTO)
            .setStatPrefix("default_inbound_config")
            .setRds(
                Rds.newBuilder()
                    .setConfigSource(
                        ConfigSource.newBuilder()
                            .setResourceApiVersion(ApiVersion.V3)
                            .setAds(AggregatedConfigSource.getDefaultInstance())
                            .build())
                    .setRouteConfigName(routeConfigName)
                    .build())
            .addHttpFilters(
                HttpFilter.newBuilder()
                    .setName(ENVOY_FILTER_HTTP_ROUTER)
                    .setTypedConfig(Any.pack(Router.newBuilder().build()))
                    .build())
            .setForwardClientCertDetails(ForwardClientCertDetails.APPEND_FORWARD)
            .setSetCurrentClientCertDetails(
                SetCurrentClientCertDetails.newBuilder()
                    .setSubject(BoolValue.of(true))
                    .setDns(true)
                    .setUri(true)
                    .build())
            .addUpgradeConfigs(UpgradeConfig.newBuilder().setUpgradeType("websocket").build())
            .build();

    String listenerName = SERVER_LISTENER_RESOURCE_NAME_TEMPLATE.formatted(address + ":" + port);
    var socketAddressAddress = address;
    if (address.startsWith("[") && address.endsWith("]")) {
      // Special IPv6 address handling ("[::]" -> "::"):
      socketAddressAddress = address.substring(1, address.length() - 1);
    }

    return Listener.newBuilder()
        .setName(listenerName)
        .setAddress(
            Address.newBuilder()
                .setSocketAddress(
                    SocketAddress.newBuilder()
                        .setAddress(socketAddressAddress)
                        .setPortValue(port)
                        .setProtocol(Protocol.TCP)
                        .build())
                .build())
        .addFilterChains(
            FilterChain.newBuilder()
                .addFilters(
                    Filter.newBuilder()
                        .setName(ENVOY_HTTP_CONNECTION_MANAGER) // must be the last filter
                        .setTypedConfig(Any.pack(httpConnectionManager))
                        .build())
                .build())
        .setTrafficDirection(TrafficDirection.INBOUND)
        .setEnableReusePort(BoolValue.of(true))
        .build();
  }

  /**
   * Application route configuration with one virtual host and one route for that virtual host.
   *
   * <p>Initial xDS implementation in grpc-java: The gRPC client will only use the last route.
   *
   * <p>Future: &quot;selected based on which RPC method is being called or possibly on a header
   * match (details TBD)&quot; (<a
   * href="https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#routeconfiguration-proto">RFC</a>).
   *
   * @param name route configuration name
   * @param virtualHostName is not used for routing
   * @param routePrefix use either <code>/</code> or <code>/[package].[service]/</code>
   * @param clusterName cluster name
   * @return the route configuration
   */
  private RouteConfiguration createRouteConfiguration(
      String name, String virtualHostName, String routePrefix, String clusterName) {
    return RouteConfiguration.newBuilder()
        .setName(name)
        .addVirtualHosts(
            VirtualHost.newBuilder()
                .setName(virtualHostName)
                .addDomains("*") // must match request `:authority`
                .addRoutes(
                    Route.newBuilder()
                        .setMatch(RouteMatch.newBuilder().setPrefix(routePrefix).build())
                        .setRoute(RouteAction.newBuilder().setCluster(clusterName).build())
                        .build())
                .build())
        .build();
  }

  /** Route configuration for the server listener(s). */
  private RouteConfiguration createRouteConfigForServerListener(String routeConfigName) {
    return RouteConfiguration.newBuilder()
        .setName(routeConfigName)
        .addVirtualHosts(
            VirtualHost.newBuilder()
                .setName("inbound|default_inbound_config-50051")
                .addDomains("*")
                .addRoutes(
                    Route.newBuilder()
                        .setMatch(RouteMatch.newBuilder().setPrefix("/").build())
                        .setDecorator(
                            Decorator.newBuilder().setOperation("default_inbound_config/*").build())
                        .setNonForwardingAction(NonForwardingAction.getDefaultInstance())
                        .build())
                .build())
        .build();
  }

  /**
   * Cluster definition for CDS.
   *
   * @see <a
   *     href="https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#cluster-proto">gRFC
   *     A27: xDS-Based Global Load Balancing</a>
   */
  private Cluster createCluster(String clusterName) {
    return Cluster.newBuilder()
        .setName(clusterName)
        .setType(DiscoveryType.EDS)
        .setEdsClusterConfig(
            EdsClusterConfig.newBuilder()
                .setEdsConfig(
                    ConfigSource.newBuilder()
                        .setResourceApiVersion(ApiVersion.V3)
                        .setAds(AggregatedConfigSource.getDefaultInstance())
                        .build())
                .build())
        .setConnectTimeout(Durations.fromSeconds(3)) // default is 5s
        .build();
  }

  /**
   * ClusterLoadAssignment definition for EDS.
   *
   * @see <a
   *     href="https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#clusterloadassignment-proto">gRFC
   *     A27: xDS-Based Global Load Balancing</a>
   */
  private ClusterLoadAssignment createClusterLoadAssignment(
      String clusterName, int port, Collection<GrpcApplicationEndpoint> endpoints) {
    var clusterLoadAssignmentBuilder =
        ClusterLoadAssignment.newBuilder().setClusterName(clusterName);

    Map<Locality, List<GrpcApplicationEndpoint>> endpointsByLocality =
        endpoints.stream()
            .collect(
                Collectors.groupingBy(
                    endpoint -> Locality.newBuilder().setZone(endpoint.zone()).build()));
    for (var locality : endpointsByLocality.keySet()) {
      var localityLbEndpointsBuilder =
          LocalityLbEndpoints.newBuilder()
              // Locality must be unique for a given priority.
              .setLocality(locality)
              // Weight is effectively mandatory, read the javadoc carefully :-)
              .setLoadBalancingWeight(UInt32Value.of(100000))
              // Priority is optional. If provided, must start from 0 and have no gaps.
              .setPriority(0);
      List<String> addressesForLocality =
          endpointsByLocality.get(locality).stream()
              .flatMap(endpoint -> endpoint.addresses().stream())
              .toList();
      for (String address : addressesForLocality) {
        // LbEndpoints is mandatory.
        localityLbEndpointsBuilder.addLbEndpoints(
            LbEndpoint.newBuilder()
                // Endpoint is mandatory.
                .setEndpoint(
                    io.envoyproxy.envoy.config.endpoint.v3.Endpoint.newBuilder()
                        // Address is mandatory, must be unique within the cluster.
                        .setAddress(
                            Address.newBuilder()
                                .setSocketAddress(
                                    SocketAddress.newBuilder()
                                        .setAddress(address) // mandatory, IPv4 or IPv6
                                        .setPortValue(port) // mandatory
                                        .setProtocolValue(Protocol.TCP_VALUE)
                                        .build())
                                .build())
                        .build())
                .setHealthStatus(HealthStatus.HEALTHY)
                .build());
      }
      clusterLoadAssignmentBuilder.addEndpoints(localityLbEndpointsBuilder.build());
    }
    return clusterLoadAssignmentBuilder.build();
  }
}
