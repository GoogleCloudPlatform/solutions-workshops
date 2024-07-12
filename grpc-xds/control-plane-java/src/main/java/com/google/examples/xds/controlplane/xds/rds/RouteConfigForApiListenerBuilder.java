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

package com.google.examples.xds.controlplane.xds.rds;

import io.envoyproxy.envoy.config.route.v3.Route;
import io.envoyproxy.envoy.config.route.v3.RouteAction;
import io.envoyproxy.envoy.config.route.v3.RouteConfiguration;
import io.envoyproxy.envoy.config.route.v3.RouteMatch;
import io.envoyproxy.envoy.config.route.v3.VirtualHost;
import org.jetbrains.annotations.NotNull;

/**
 * Builds RDS RouteConfigurations for LDS Listeners of type <a
 * href="https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/listener/v3/api_listener.proto">API
 * listener</a>.
 *
 * <p>This builder creates a RouteConfiguration with one VirtualHost that has a wildcard Domain and
 * one Route.
 *
 * <p>API listeners are used by gRPC clients for upstream connections.
 */
public class RouteConfigForApiListenerBuilder {
  private final String name;
  private String virtualHostName;
  private String routePrefix;
  private String clusterName;

  /**
   * Sets the RDS RouteConfiguration name.
   *
   * @param name route configuration name
   */
  public RouteConfigForApiListenerBuilder(@NotNull String name) {
    if (name.isBlank()) {
      throw new IllegalArgumentException("RouteConfiguration name must not be blank");
    }
    this.name = name;
    this.virtualHostName = name;
    this.routePrefix = "";
    this.clusterName = name;
  }

  /**
   * Sets the virtual host name. The Listener created by this builder has only one virtual host.
   *
   * @param virtualHostName is not used for routing
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public RouteConfigForApiListenerBuilder withVirtualHostName(@NotNull String virtualHostName) {
    if (virtualHostName.isBlank()) {
      throw new IllegalArgumentException("RouteConfiguration virtualHostName must not be blank");
    }
    this.virtualHostName = virtualHostName;
    return this;
  }

  /**
   * Sets the prefix for matching the request URI path for the lone route.
   *
   * @param routePrefix is used for URI path prefix matching, can be empty string
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public RouteConfigForApiListenerBuilder withRoutePrefix(@NotNull String routePrefix) {
    this.routePrefix = routePrefix;
    return this;
  }

  /** The CDS Cluster to route requests to. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public RouteConfigForApiListenerBuilder withClusterName(@NotNull String clusterName) {
    if (clusterName.isBlank()) {
      throw new IllegalArgumentException("RouteConfiguration clusterName must not be blank");
    }
    this.clusterName = clusterName;
    return this;
  }

  /** Builds the RDS RouteConfiguration. */
  @NotNull
  public RouteConfiguration build() {
    var builder =
        RouteConfiguration.newBuilder()
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
                    .build());
    return builder.build();
  }
}
