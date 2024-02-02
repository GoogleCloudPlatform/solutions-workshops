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
import io.envoyproxy.envoy.config.core.v3.TransportSocket;
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
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CertificateProviderPluginInstance;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CertificateValidationContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CommonTlsContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.Secret;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext;
import io.envoyproxy.envoy.type.matcher.v3.RegexMatcher;
import io.envoyproxy.envoy.type.matcher.v3.StringMatcher;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

/** Builds xDS resource snapshots for the cache. */
public class XdsSnapshotBuilder {

  /**
   * Listener name template used xDS clients that are gRPC servers.
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
  static final String SERVER_LISTENER_RESOURCE_NAME_TEMPLATE =
      "grpc/server?xds.resource.listening_address=%s";

  /** Copied from {@link io.grpc.xds.XdsListenerResource}. */
  @SuppressWarnings("JavadocReference")
  private static final String TRANSPORT_SOCKET_NAME_TLS = "envoy.transport_sockets.tls";

  /** Copied from {@link io.envoyproxy.controlplane.cache.Resources}. */
  private static final String ENVOY_HTTP_CONNECTION_MANAGER = "envoy.http_connection_manager";

  /** Copied from {@link io.envoyproxy.controlplane.cache.Resources}. */
  private static final String ENVOY_FILTER_HTTP_ROUTER = "envoy.filters.http.router";

  private static final String ENVOY_FILTER_HTTP_FAULT = "envoy.filters.http.fault";

  /**
   * Used for the RouteConfiguration pointed to by server Listeners.
   *
   * <p>Different server Listeners can point to the same RouteConfiguration, since the
   * RouteConfiguration does not contain the listening address or port.
   */
  private static final String SERVER_LISTENER_ROUTE_CONFIG_NAME = "default_inbound_config";

  /**
   * TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME is used in the `[Down|Up]streamTlsContext`s.
   *
   * <p>Using the same name as the <a
   * href="https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/2a9cf4614b56ec085c391a12f4cc53defaa575ac/main.go#L276">
   * <code>traffic-director-grpc-bootstrap</code></a> tool, but this is not important.
   */
  private static final String TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME =
      "google_cloud_private_spiffe";

  private final Map<String, Listener> listeners = new HashMap<>();
  private final Map<String, RouteConfiguration> routeConfigurations = new HashMap<>();
  private final Map<String, Cluster> clusters = new HashMap<>();
  private final Map<String, ClusterLoadAssignment> clusterLoadAssignments = new HashMap<>();

  /** Addresses for server listeners to be added to the snapshot. */
  private final Set<EndpointAddress> serverListenerAddresses = new HashSet<>();

  private final XdsFeatures xdsFeatures;

  /** Creates a builder for an xDS resource cache snapshot. */
  public XdsSnapshotBuilder(@NotNull XdsFeatures xdsFeatures) {
    this.xdsFeatures = xdsFeatures;
  }

  /**
   * Add the Listener, RouteConfiguration, Cluster, and ClusterLoadAssignment resources from the
   * provided snapshot to the builder.
   */
  @SuppressWarnings("UnusedReturnValue")
  @NotNull
  public XdsSnapshotBuilder addSnapshot(@Nullable Snapshot snapshot) {
    if (snapshot == null) {
      return this;
    }
    if (snapshot.listeners() != null && snapshot.listeners().resources() != null) {
      listeners.putAll(snapshot.listeners().resources());
    }
    if (snapshot.routes() != null && snapshot.routes().resources() != null) {
      routeConfigurations.putAll(snapshot.routes().resources());
    }
    if (snapshot.clusters() != null && snapshot.clusters().resources() != null) {
      clusters.putAll(snapshot.clusters().resources());
    }
    if (snapshot.endpoints() != null && snapshot.endpoints().resources() != null) {
      clusterLoadAssignments.putAll(snapshot.endpoints().resources());
    }
    return this;
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
  @NotNull
  public XdsSnapshotBuilder addGrpcApplications(@NotNull Set<GrpcApplication> apps) {
    for (GrpcApplication app : apps) {
      Listener listener = createApiListener(app.listenerName(), app.routeName());
      listeners.put(listener.getName(), listener);
      RouteConfiguration routeConfiguration =
          createRouteConfiguration(
              app.routeName(), app.listenerName(), app.pathPrefix(), app.clusterName());
      routeConfigurations.put(routeConfiguration.getName(), routeConfiguration);
      Cluster cluster = createCluster(app.clusterName(), app.namespace(), app.serviceAccountName());
      clusters.put(cluster.getName(), cluster);
      ClusterLoadAssignment clusterLoadAssignment =
          createClusterLoadAssignment(app.clusterName(), app.port(), app.endpoints());
      clusterLoadAssignments.put(clusterLoadAssignment.getClusterName(), clusterLoadAssignment);
    }
    return this;
  }

  @NotNull
  @SuppressWarnings("UnusedReturnValue")
  public XdsSnapshotBuilder addServerListenerAddresses(
      @NotNull Collection<EndpointAddress> addresses) {
    serverListenerAddresses.addAll(addresses);
    return this;
  }

  /** Builds an xDS resource snapshot. */
  @NotNull
  public Snapshot build() {
    for (EndpointAddress address : serverListenerAddresses) {
      Listener serverListener =
          createServerListener(
              address.ipAddress(),
              address.port(),
              xdsFeatures.serverListenerUsesRds(),
              xdsFeatures.enableDataPlaneTls(),
              xdsFeatures.requireDataPlaneClientCerts());
      listeners.put(serverListener.getName(), serverListener);
    }
    if (!serverListenerAddresses.isEmpty()) {
      RouteConfiguration routeConfigForServerListener = createRouteConfigForServerListener();
      routeConfigurations.put(routeConfigForServerListener.getName(), routeConfigForServerListener);
    }
    ImmutableList<Secret> secrets = ImmutableList.of();
    String version = String.valueOf(System.nanoTime());
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
  private Listener createApiListener(@NotNull String listenerName, @NotNull String routeName) {
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
   * Create the server listener for xDS-enabled gRPC servers.
   *
   * @return a Listener, using RDS for the RouteConfiguration
   */
  private Listener createServerListener(
      @NotNull String address,
      int port,
      boolean useRds,
      boolean enableTls,
      boolean requireClientCerts) {
    var httpConnectionManager = createHttpConnectionManagerForServerListener(useRds);

    var filterChainBuilder =
        FilterChain.newBuilder()
            .addFilters(
                Filter.newBuilder()
                    .setName(ENVOY_HTTP_CONNECTION_MANAGER) // must be the last filter
                    .setTypedConfig(Any.pack(httpConnectionManager))
                    .build());
    if (enableTls) {
      var downstreamTlsContext = createDownstreamTlsContext(requireClientCerts);
      filterChainBuilder.setTransportSocket(
          TransportSocket.newBuilder()
              .setName(TRANSPORT_SOCKET_NAME_TLS)
              .setTypedConfig(Any.pack(downstreamTlsContext))
              .build());
    }
    var filterChain = filterChainBuilder.build();

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
        .addFilterChains(filterChain)
        .setTrafficDirection(TrafficDirection.INBOUND)
        .setEnableReusePort(BoolValue.of(true))
        .build();
  }

  private HttpConnectionManager createHttpConnectionManagerForServerListener(boolean useRds) {
    var httpConnectionManagerBuilder =
        HttpConnectionManager.newBuilder()
            .setCodecType(CodecType.AUTO)
            .setStatPrefix("default_inbound_config")
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
            .addUpgradeConfigs(UpgradeConfig.newBuilder().setUpgradeType("websocket").build());

    // Use setRouteConfig() for inline RouteConfiguration, or setRds() for dynamic RDS.
    if (useRds) {
      httpConnectionManagerBuilder.setRds(
          Rds.newBuilder()
              .setConfigSource(
                  ConfigSource.newBuilder()
                      .setResourceApiVersion(ApiVersion.V3)
                      .setAds(AggregatedConfigSource.getDefaultInstance())
                      .build())
              .setRouteConfigName(SERVER_LISTENER_ROUTE_CONFIG_NAME)
              .build());
    } else {
      httpConnectionManagerBuilder.setRouteConfig(createRouteConfigForServerListener());
    }

    return httpConnectionManagerBuilder.build();
  }

  private DownstreamTlsContext createDownstreamTlsContext(boolean requireClientCerts) {
    var commonTlsContextBuilder =
        CommonTlsContext.newBuilder()
            // Set server certificate:
            .setTlsCertificateProviderInstance(
                CertificateProviderPluginInstance.newBuilder()
                    // https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/2a9cf4614b56ec085c391a12f4cc53defaa575ac/main.go#L276
                    .setInstanceName(TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME)
                    // Using the same certificate name value as Traffic Director, but the
                    // certificate name is ignored by gRPC according to gRFC A29.
                    .setCertificateName("DEFAULT")
                    .build())
            // Traffic Director sets `alpn_protocols`, but it is ignored by gRPC according to gRFC
            // A29.
            .addAlpnProtocols("h2");

    var downstreamTlsContextBuilder = DownstreamTlsContext.newBuilder();

    if (requireClientCerts) {
      // `require_client_certificate: true` requires a `validation_context`.
      downstreamTlsContextBuilder.setRequireClientCertificate(BoolValue.of(true));
      // Validate client certificates:
      // gRFC A29 specifies to use either `validation_context` or
      // `combined_validation_context.default_validation_context`, but
      // gRPC-Java as of v1.60.0 doesn't handle `combined_validation_context` correctly.
      commonTlsContextBuilder.setValidationContext(
          CertificateValidationContext.newBuilder()
              .setCaCertificateProviderInstance(
                  CertificateProviderPluginInstance.newBuilder()
                      .setInstanceName(TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME)
                      // Using the same certificate name value as Traffic Director,
                      // but the certificate name is ignored by gRPC, see gRFC A29.
                      .setCertificateName("ROOTCA")
                      .build())
              .build());
    }

    downstreamTlsContextBuilder.setCommonTlsContext(commonTlsContextBuilder.build());

    return downstreamTlsContextBuilder.build();
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
      @NotNull String name,
      @NotNull String virtualHostName,
      @NotNull String routePrefix,
      @NotNull String clusterName) {
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

  /** Route configuration for the server listeners. */
  private RouteConfiguration createRouteConfigForServerListener() {
    return RouteConfiguration.newBuilder()
        .setName(SERVER_LISTENER_ROUTE_CONFIG_NAME)
        .addVirtualHosts(
            VirtualHost.newBuilder()
                // The VirtualHost name _doesn't_ have to match the RouteConfiguration name.
                .setName(SERVER_LISTENER_ROUTE_CONFIG_NAME)
                .addDomains("*")
                .addRoutes(
                    Route.newBuilder()
                        .setMatch(RouteMatch.newBuilder().setPrefix("/").build())
                        .setDecorator(
                            Decorator.newBuilder()
                                .setOperation(SERVER_LISTENER_ROUTE_CONFIG_NAME + "/*")
                                .build())
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
  private Cluster createCluster(
      @NotNull String clusterName, @NotNull String namespace, @NotNull String serviceAccountName) {
    //noinspection deprecation
    var upstreamTlsContext =
        UpstreamTlsContext.newBuilder()
            .setCommonTlsContext(
                CommonTlsContext.newBuilder()
                    // Send client certificate in TLS handshake:
                    .setTlsCertificateProviderInstance(
                        CertificateProviderPluginInstance.newBuilder()
                            .setInstanceName(TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME)
                            // Using the same certificate name value as Traffic Director,
                            // but the certificate name is ignored by gRPC says gRFC A29.
                            .setCertificateName("DEFAULT")
                            .build())
                    // Validate gRPC server certificate:
                    // gRFC A29 specifies to use either `validation_context` or
                    // `combined_validation_context.default_validation_context`, but
                    // gRPC-Java as of v1.60.0 doesn't handle `combined_validation_context`
                    // correctly.
                    .setValidationContext(
                        CertificateValidationContext.newBuilder()
                            .setCaCertificateProviderInstance(
                                CertificateProviderPluginInstance.newBuilder()
                                    .setInstanceName(TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME)
                                    // Using the same certificate name value as Traffic Director,
                                    // but the certificate name is ignored by gRPC says gRFC A29.
                                    .setCertificateName("ROOTCA")
                                    .build())
                            // Server authorization (SAN checks):
                            // gRPC-Java as of v1.60.0 does not work correctly with
                            // `match_typed_subject_alt_names`, using `match_subject_alt_names`
                            // instead, for now.
                            .addMatchSubjectAltNames(
                                StringMatcher.newBuilder()
                                    .setSafeRegex(
                                        RegexMatcher.newBuilder()
                                            .setRegex(
                                                "spiffe://[^/]+/ns/%s/sa/%s"
                                                    .formatted(namespace, serviceAccountName))
                                            .build())
                                    .build())
                            .build())
                    // Traffic Director sets `alpn_protocols`, but it is ignored by gRPC according
                    // to gRFC A29.
                    .addAlpnProtocols("h2")
                    .build())
            .build();

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
        .setTransportSocket(
            TransportSocket.newBuilder()
                .setName(TRANSPORT_SOCKET_NAME_TLS)
                .setTypedConfig(Any.pack(upstreamTlsContext))
                .build())
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
      @NotNull String clusterName,
      int port,
      @NotNull Collection<GrpcApplicationEndpoint> endpoints) {
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
