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

package com.google.examples.xds.controlplane.xds.lds;

import com.google.protobuf.Any;
import com.google.protobuf.BoolValue;
import io.envoyproxy.envoy.config.core.v3.AggregatedConfigSource;
import io.envoyproxy.envoy.config.core.v3.ApiVersion;
import io.envoyproxy.envoy.config.core.v3.ConfigSource;
import io.envoyproxy.envoy.extensions.filters.http.fault.v3.HTTPFault;
import io.envoyproxy.envoy.extensions.filters.http.rbac.v3.RBAC;
import io.envoyproxy.envoy.extensions.filters.http.router.v3.Router;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpFilter;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.Rds;
import org.jetbrains.annotations.NotNull;

/**
 * Builds <a
 * href="https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/http_conn_man">HTTP
 * connection managers</a> for LDS listeners.
 */
public class HttpConnectionManagerBuilder {

  /** Copied from {@link io.envoyproxy.controlplane.cache.Resources}. */
  private static final String ENVOY_FILTER_HTTP_ROUTER = "envoy.filters.http.router";

  public static final String ENVOY_FILTER_HTTP_FAULT = "envoy.filters.http.fault";
  public static final String ENVOY_FILTER_HTTP_RBAC = "envoy.filters.http.rbac";

  /**
   * Builds HTTP connection managers for LDS socket Listeners, to be used by
   * gRPC servers and Envoy proxy instances.
   */
  public static class ForSocketListener {
    private final String routeConfigurationName;
    private String statPrefix;
    private boolean enableRbac;

    /** RouteConfiguration name is used with RDS. */
    public ForSocketListener(@NotNull String routeConfigurationName) {
      if (routeConfigurationName.isBlank()) {
        throw new IllegalArgumentException(
            "Listener HttpConnectionManager routeConfigurationName must not be blank");
      }
      this.routeConfigurationName = routeConfigurationName;
      this.statPrefix = "default_inbound_config";
    }

    /** Human-readable prefix used when emitting statistics. */
    @SuppressWarnings({"unused", "UnusedReturnValue"})
    @NotNull
    public ForSocketListener withStatPrefix(@NotNull String statPrefix) {
      if (statPrefix.isBlank()) {
        throw new IllegalArgumentException(
            "Listener HttpConnectionManager statPrefix must not be blank");
      }
      this.statPrefix = statPrefix;
      return this;
    }

    /** Enable client workload authorization. */
    @SuppressWarnings({"unused", "UnusedReturnValue"})
    @NotNull
    public ForSocketListener withEnableRbac(boolean enableRbac) {
      this.enableRbac = enableRbac;
      return this;
    }

    /** Builds the HttpConnectionManager. */
    @NotNull
    public HttpConnectionManager build() {
      var httpConnectionManagerBuilder =
          HttpConnectionManager.newBuilder()
              .setCodecType(HttpConnectionManager.CodecType.AUTO)
              // https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/stats#config-http-conn-man-stats
              .setStatPrefix(statPrefix)
              .setRds(
                  Rds.newBuilder()
                      .setConfigSource(
                          ConfigSource.newBuilder()
                              .setResourceApiVersion(ApiVersion.V3)
                              .setAds(AggregatedConfigSource.getDefaultInstance())
                              .build())
                      .setRouteConfigName(routeConfigurationName)
                      .build())
              .addHttpFilters(
                  // Router must be the last filter.
                  HttpFilter.newBuilder()
                      .setName(ENVOY_FILTER_HTTP_ROUTER)
                      .setTypedConfig(
                          Any.pack(Router.newBuilder().setSuppressEnvoyHeaders(false).build()))
                      .build())
              .setForwardClientCertDetails(
                  HttpConnectionManager.ForwardClientCertDetails.APPEND_FORWARD)
              .setSetCurrentClientCertDetails(
                  HttpConnectionManager.SetCurrentClientCertDetails.newBuilder()
                      .setSubject(BoolValue.of(true))
                      .setDns(true)
                      .setUri(true)
                      .build())
              .addUpgradeConfigs(
                  HttpConnectionManager.UpgradeConfig.newBuilder()
                      .setUpgradeType("websocket")
                      .build());

      if (enableRbac) {
        // Prepend RBAC HTTP filter. Not append, as Router must be the last HTTP filter.
        httpConnectionManagerBuilder.addHttpFilters(
            0,
            HttpFilter.newBuilder()
                .setName(ENVOY_FILTER_HTTP_RBAC)
                .setTypedConfig(
                    Any.pack(
                        RBAC.newBuilder()
                            // Present and empty `Rules` mean DENY all. Override per route.
                            .setRules(io.envoyproxy.envoy.config.rbac.v3.RBAC.newBuilder().build())
                            .build()))
                .build());
      }
      return httpConnectionManagerBuilder.build();
    }
  }

  /**
   * Builds HTTP connection managers for LDS API Listeners, to be used by gRPC clients.
   */
  public static class ForApiListener {
    private final String routeConfigurationName;
    private String statPrefix;

    /** RouteConfiguration can just be the upstream application name, as an example. */
    public ForApiListener(@NotNull String routeConfigurationName) {
      if (routeConfigurationName.isBlank()) {
        throw new IllegalArgumentException(
            "Listener HttpConnectionManager routeConfigurationName must not be blank");
      }
      this.routeConfigurationName = routeConfigurationName;
      this.statPrefix = routeConfigurationName;
    }

    /** Human-readable prefix used when emitting statistics. */
    @SuppressWarnings({"unused", "UnusedReturnValue"})
    @NotNull
    public ForApiListener withStatPrefix(@NotNull String statPrefix) {
      if (statPrefix.isBlank()) {
        throw new IllegalArgumentException(
            "Listener HttpConnectionManager statPrefix must not be blank");
      }
      this.statPrefix = statPrefix;
      return this;
    }

    /** Builds the HttpConnectionManager. */
    @NotNull
    public HttpConnectionManager build() {
      var httpConnectionManagerBuilder =
          HttpConnectionManager.newBuilder()
              .setCodecType(HttpConnectionManager.CodecType.AUTO)
              // https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/stats#config-http-conn-man-stats
              .setStatPrefix(statPrefix)
              .setRds(
                  Rds.newBuilder()
                      .setConfigSource(
                          ConfigSource.newBuilder()
                              .setResourceApiVersion(ApiVersion.V3)
                              .setAds(AggregatedConfigSource.getDefaultInstance())
                              .build())
                      .setRouteConfigName(routeConfigurationName)
                      .build())
              .addHttpFilters(
                  // Enable client-side fault injection.
                  HttpFilter.newBuilder()
                      .setName(ENVOY_FILTER_HTTP_FAULT)
                      .setTypedConfig(Any.pack(HTTPFault.getDefaultInstance()))
                      .build())
              .addHttpFilters(
                  // Router must be the last filter.
                  HttpFilter.newBuilder()
                      .setName(ENVOY_FILTER_HTTP_ROUTER)
                      .setTypedConfig(Any.pack(Router.newBuilder().build()))
                      .build());
      return httpConnectionManagerBuilder.build();
    }
  }
}
