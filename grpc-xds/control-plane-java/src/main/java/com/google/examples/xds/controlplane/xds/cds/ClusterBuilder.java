// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.examples.xds.controlplane.xds.cds;

import com.google.examples.xds.controlplane.xds.tls.TransportSocketBuilder;
import com.google.examples.xds.controlplane.xds.tls.UpstreamTlsContextBuilder;
import com.google.protobuf.Any;
import com.google.protobuf.Duration;
import com.google.protobuf.UInt32Value;
import com.google.protobuf.util.Durations;
import io.envoyproxy.envoy.config.cluster.v3.Cluster;
import io.envoyproxy.envoy.config.cluster.v3.Cluster.DiscoveryType;
import io.envoyproxy.envoy.config.cluster.v3.Cluster.EdsClusterConfig;
import io.envoyproxy.envoy.config.cluster.v3.Cluster.LbPolicy;
import io.envoyproxy.envoy.config.core.v3.AggregatedConfigSource;
import io.envoyproxy.envoy.config.core.v3.ApiVersion;
import io.envoyproxy.envoy.config.core.v3.ConfigSource;
import io.envoyproxy.envoy.config.core.v3.HealthCheck;
import io.envoyproxy.envoy.config.core.v3.HealthCheck.GrpcHealthCheck;
import io.envoyproxy.envoy.config.core.v3.HealthCheck.HttpHealthCheck;
import io.envoyproxy.envoy.config.core.v3.HealthCheck.TcpHealthCheck;
import io.envoyproxy.envoy.config.core.v3.Http2ProtocolOptions;
import io.envoyproxy.envoy.extensions.upstreams.http.v3.HttpProtocolOptions;
import io.envoyproxy.envoy.extensions.upstreams.http.v3.HttpProtocolOptions.ExplicitHttpConfig;
import java.util.Set;
import org.jetbrains.annotations.NotNull;

/**
 * Cluster definition for CDS.
 *
 * @see <a
 *     href="https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#cluster-proto">gRFC
 *     A27: xDS-Based Global Load Balancing</a>
 */
public class ClusterBuilder {
  /** TODO: Make these configurable. */
  private static final int HEALTH_CHECK_INTERVAL_SECONDS = 30;

  private static final int HEALTH_CHECK_TIMEOUT_SECONDS = 1;

  /**
   * Key for HTTP TypedExtensionProtocolOptions, required by Envoy for HTTP/2 to upstream clusters.
   */
  private static final String ENVOY_EXTENSIONS_UPSTREAMS_HTTP_PROTOCOL_OPTIONS =
      "envoy.extensions.upstreams.http.v3.HttpProtocolOptions";

  /** Accepted values for CDS Cluster-level client-side active health checks (for Envoy). */
  private static final Set<String> HEALTH_CHECK_TYPES = Set.of("tcp", "http", "grpc");

  private final String name;
  private String edsServiceName;
  private int connectTimeoutSeconds;
  private boolean enableTls;
  private boolean enableServerAuthorization;
  private String trustDomain;
  private String namespace;
  private String serviceAccount;
  private boolean requireClientCerts;
  private String healthCheckProtocol;
  private int healthCheckPort;
  private String healthCheckPathOrGrpcService;

  /** CDS Clusters must have a name. */
  public ClusterBuilder(@NotNull String name) {
    if (name.isBlank()) {
      throw new IllegalArgumentException("Cluster name must not be blank");
    }
    this.name = name;
    this.edsServiceName = name;
    this.connectTimeoutSeconds = 5; // the default if unspecified is 5 according to the spec
    this.trustDomain = "[^/]+";
    this.namespace = "[^/]+";
    this.serviceAccount = ".+";
  }

  /**
   * Sets the resource name to request from EDS. Typically, this is just the CDS Cluster name, but
   * it must be a different name if the CDS Cluster name uses the <code>xdstp://</code> scheme for
   * xDS federation.
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ClusterBuilder withEdsServiceName(@NotNull String edsServiceName) {
    if (edsServiceName.isBlank()) {
      throw new IllegalArgumentException("Cluster EDS service name must not be blank");
    }
    this.edsServiceName = edsServiceName;
    return this;
  }

  /** Override default connection timeout. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ClusterBuilder withConnectTimeoutSeconds(int connectTimeoutSeconds) {
    if (connectTimeoutSeconds <= 0) {
      throw new IllegalArgumentException(
          "CDS Cluster connectTimeoutSeconds must be greater than 0");
    }
    this.connectTimeoutSeconds = connectTimeoutSeconds;
    return this;
  }

  /** Use TLS, and enable validation of upstream (server) TLS certificate. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ClusterBuilder withEnableTls(boolean enableTls) {
    this.enableTls = enableTls;
    return this;
  }

  /** Enables mTLS, also sets {@link #enableTls} to <code>true</code>. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ClusterBuilder withRequireClientCerts(boolean requireClientCerts) {
    this.enableTls = true;
    this.requireClientCerts = requireClientCerts;
    return this;
  }

  /**
   * Use if clients should authorize servers according to <a
   * href="https://github.com/grpc/proposal/blob/deaf1bcf248d1e48e83c470b00930cbd363fab6d/A29-xds-tls-security.md#server-authorization-aka-subject-alt-name-checks">gRFC
   * A29</a>.
   *
   * <p>This creates a SAN matcher in the UpstreamTLSContext that matches the SPIFFE ID in the
   * server's certificate against the pattern <code>
   * spiffe://[trustDomain]/ns/[namespace]/sa/[serviceAccount]</code>.
   *
   * @param trustDomain the SPIFFE trust domain to match, use <code>&quot;[^/]+&quot;</code> to
   *     match any trust domain
   * @param namespace the k8s namespace to match, use <code>&quot;[^/]+&quot;</code> to match any
   *     namespace
   * @param serviceAccount k8s service account name to match, use <code>&quot;.+&quot;</code> to
   *     match any service account
   * @return the Cluster builder
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ClusterBuilder withServerAuthorization(
      @NotNull String trustDomain, @NotNull String namespace, @NotNull String serviceAccount) {
    if (trustDomain.isBlank() || namespace.isBlank() || serviceAccount.isBlank()) {
      throw new IllegalArgumentException(
          "CDS Cluster trustDomain, namespace, and serviceAccount must all be non-blank");
    }
    this.enableServerAuthorization = true;
    this.trustDomain = trustDomain;
    this.namespace = namespace;
    this.serviceAccount = serviceAccount;
    return this;
  }

  /**
   * Configure client-side active health check for CDS Cluster endpoints.
   *
   * <p>Client-side active health checks are supported by Envoy proxy, but not by gRPC clients.
   *
   * @param protocol for the health check, valid values are "tcp", "http", and "grpc"
   * @param port health check port, use <code>0</code> for health checks on the serving port.
   * @param pathOrGrpcService URL path for HTTP health checks (e.g., "/healthz"), or gRPC service
   *     name for gRPC health checks (e.g., "helloworld.Greeter"). Unused for TCP health checks. The
   *     value can be an empty string.
   * @return the Cluster builder
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ClusterBuilder withHealthCheck(
      @NotNull String protocol, int port, @NotNull String pathOrGrpcService) {
    if (!HEALTH_CHECK_TYPES.contains(protocol)) {
      throw new IllegalArgumentException(
          "Unsupported health check protocol ["
              + protocol
              + "], must be one of "
              + HEALTH_CHECK_TYPES);
    }
    this.healthCheckProtocol = protocol;
    this.healthCheckPort = port;
    this.healthCheckPathOrGrpcService = pathOrGrpcService;
    return this;
  }

  /** Creates the CDS Cluster resource. */
  public Cluster build() {
    var clusterBuilder =
        Cluster.newBuilder()
            .setName(name)
            .setType(DiscoveryType.EDS)
            .setEdsClusterConfig(
                EdsClusterConfig.newBuilder()
                    .setEdsConfig(
                        ConfigSource.newBuilder()
                            .setResourceApiVersion(ApiVersion.V3)
                            .setAds(AggregatedConfigSource.getDefaultInstance())
                            .build())
                    // ServiceName is the name to use for EDS ClusterLoadAssignment lookup. If
                    // unspecified, the cluster name is used for EDS. ServiceName is required when
                    // the cluster name uses the xdstp:// scheme (for xDS federation), otherwise it
                    // is optional.
                    .setServiceName(edsServiceName)
                    .build())
            .setConnectTimeout(Durations.fromSeconds(connectTimeoutSeconds))
            .putTypedExtensionProtocolOptions(
                ENVOY_EXTENSIONS_UPSTREAMS_HTTP_PROTOCOL_OPTIONS,
                Any.pack(
                    HttpProtocolOptions.newBuilder()
                        .setExplicitHttpConfig(
                            ExplicitHttpConfig.newBuilder()
                                .setHttp2ProtocolOptions(Http2ProtocolOptions.newBuilder().build())
                                .build())
                        .build()))
            // Client-side active health checks. Implemented by Envoy, but not by gRPC clients.
            // See
            // https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/service_discovery#on-eventually-consistent-service-discovery
            // and https://github.com/grpc/grpc/issues/34581
            .addHealthChecks(createHealthCheck())
            // See https://github.com/envoyproxy/envoy/issues/11527
            .setIgnoreHealthOnHostRemoval(true)
            .setLbPolicy(LbPolicy.ROUND_ROBIN);

    if (enableTls) {
      var upstreamTlsContextBuilder =
          new UpstreamTlsContextBuilder().withRequireClientCerts(requireClientCerts);
      if (enableServerAuthorization) {
        upstreamTlsContextBuilder.withServerAuthorization(trustDomain, namespace, serviceAccount);
      }
      clusterBuilder.setTransportSocket(
          new TransportSocketBuilder(upstreamTlsContextBuilder.build()).build());
    }

    return clusterBuilder.build();
  }

  /**
   * Creates a CDS Cluster-level health check, based on the health check protocol.
   *
   * <p>Defaults to TCP health checks on the serving port if health check fields are not set on the
   * ClusterBuilder.
   */
  @NotNull
  private HealthCheck createHealthCheck() {
    var builder =
        HealthCheck.newBuilder()
            .setAltPort(UInt32Value.of(healthCheckPort))
            .setHealthyThreshold(UInt32Value.of(1))
            .setInterval(Duration.newBuilder().setSeconds(HEALTH_CHECK_INTERVAL_SECONDS).build())
            .setTimeout(Duration.newBuilder().setSeconds(HEALTH_CHECK_TIMEOUT_SECONDS).build())
            .setUnhealthyThreshold(UInt32Value.of(1));
    if ("grpc".equalsIgnoreCase(healthCheckProtocol)) {
      builder.setGrpcHealthCheck(
          GrpcHealthCheck.newBuilder().setServiceName(healthCheckPathOrGrpcService).build());
    } else if ("http".equalsIgnoreCase(healthCheckProtocol)) {
      builder.setHttpHealthCheck(
          HttpHealthCheck.newBuilder().setPath(healthCheckPathOrGrpcService).build());
    } else {
      // TCP fallback
      builder.setTcpHealthCheck(TcpHealthCheck.newBuilder().build());
    }
    return builder.build();
  }
}
