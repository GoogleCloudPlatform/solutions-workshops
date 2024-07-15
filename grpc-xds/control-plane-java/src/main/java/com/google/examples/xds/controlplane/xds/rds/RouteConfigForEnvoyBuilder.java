// Copyright 2024 Google LLC
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

package com.google.examples.xds.controlplane.xds.rds;

import com.google.examples.xds.controlplane.xds.lds.EnvoyListenerBuilder;
import io.envoyproxy.envoy.config.route.v3.Route;
import io.envoyproxy.envoy.config.route.v3.RouteAction;
import io.envoyproxy.envoy.config.route.v3.RouteConfiguration;
import io.envoyproxy.envoy.config.route.v3.RouteMatch;
import io.envoyproxy.envoy.config.route.v3.VirtualHost;
import java.util.Collection;
import java.util.Set;
import org.jetbrains.annotations.NotNull;

/** Builds RDS route configuration for an Envoy proxy Listener that listens for gRPC requests. */
public class RouteConfigForEnvoyBuilder {

  private final Set<String> clusterNames;

  public RouteConfigForEnvoyBuilder(@NotNull Collection<String> clusterNames) {
    this.clusterNames = Set.copyOf(clusterNames);
  }

  /** Builds the RDS RouteConfiguration. */
  @NotNull
  public RouteConfiguration build() {
    var routeConfigBuilder =
        RouteConfiguration.newBuilder()
            .setName(EnvoyListenerBuilder.ENVOY_ROUTE_CONFIGURATION_NAME);

    clusterNames.stream()
        .filter(clusterName -> !clusterName.startsWith("xdstp://"))
        .forEach(
            clusterName ->
                routeConfigBuilder.addVirtualHosts(
                    VirtualHost.newBuilder()
                        .setName(clusterName)
                        .addDomains(clusterName)
                        .addDomains(clusterName + ".example.com")
                        .addDomains(clusterName + ".xds.example.com")
                        .addRoutes(
                            Route.newBuilder()
                                .setMatch(RouteMatch.newBuilder().setPrefix("").build())
                                .setRoute(RouteAction.newBuilder().setCluster(clusterName).build())
                                .build())
                        .build()));

    return routeConfigBuilder.build();
  }
}
