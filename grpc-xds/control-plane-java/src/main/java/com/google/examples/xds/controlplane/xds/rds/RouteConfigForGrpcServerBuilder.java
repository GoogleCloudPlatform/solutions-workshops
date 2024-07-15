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

import com.google.examples.xds.controlplane.xds.lds.HttpConnectionManagerBuilder;
import com.google.protobuf.Any;
import io.envoyproxy.envoy.config.rbac.v3.Permission;
import io.envoyproxy.envoy.config.rbac.v3.Policy;
import io.envoyproxy.envoy.config.rbac.v3.Principal;
import io.envoyproxy.envoy.config.route.v3.Decorator;
import io.envoyproxy.envoy.config.route.v3.NonForwardingAction;
import io.envoyproxy.envoy.config.route.v3.Route;
import io.envoyproxy.envoy.config.route.v3.RouteConfiguration;
import io.envoyproxy.envoy.config.route.v3.RouteMatch;
import io.envoyproxy.envoy.config.route.v3.VirtualHost;
import io.envoyproxy.envoy.extensions.filters.http.rbac.v3.RBAC;
import io.envoyproxy.envoy.extensions.filters.http.rbac.v3.RBACPerRoute;
import io.envoyproxy.envoy.type.matcher.v3.PathMatcher;
import io.envoyproxy.envoy.type.matcher.v3.RegexMatcher;
import io.envoyproxy.envoy.type.matcher.v3.StringMatcher;
import org.jetbrains.annotations.NotNull;

/**
 * Builds RDS RouteConfigurations for LDS Listeners used by gRPC servers for downstream connections.
 *
 * <p>This builder creates a RouteConfiguration with one VirtualHost that has a wildcard Domain and
 * one Route, with the <code>nonForwardingAction</code> attribute set.
 */
public class RouteConfigForGrpcServerBuilder {

  /**
   * Used for the RouteConfiguration pointed to by gRPC server Listeners.
   *
   * <p>Different server Listeners can point to the same RouteConfiguration, since the
   * RouteConfiguration does not contain the listening address or servingPort.
   */
  public static final String ROUTE_CONFIGURATION_NAME_FOR_GRPC_SERVER_LISTENER =
      "default_inbound_config";

  private final String name = ROUTE_CONFIGURATION_NAME_FOR_GRPC_SERVER_LISTENER;
  private final boolean enableRbac;
  private String virtualHostName;
  private String routePrefix;
  private String decorator;

  /** RBAC is client workload authorization. */
  public RouteConfigForGrpcServerBuilder(boolean enableRbac) {
    this.enableRbac = enableRbac;
    // virtualHost name _doesn't_ have to match the RouteConfiguration name.
    this.virtualHostName = name;
    this.routePrefix = "";
    this.decorator = name + "/*";
  }

  /**
   * Sets the virtual host name. The Listener created by this builder has only one virtual host.
   *
   * @param virtualHostName is not used for routing
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public RouteConfigForGrpcServerBuilder withVirtualHostName(@NotNull String virtualHostName) {
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
  public RouteConfigForGrpcServerBuilder withRoutePrefix(@NotNull String routePrefix) {
    this.routePrefix = routePrefix;
    return this;
  }

  /**
   * Sets decorator used for tracing.
   *
   * @param decorator is used for tracing, see <a
   *     href="https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/route/v3/route_components.proto#envoy-v3-api-msg-config-route-v3-decorator">the
   *     Envoy documentation</a>
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public RouteConfigForGrpcServerBuilder withDecorator(@NotNull String decorator) {
    this.decorator = decorator;
    return this;
  }

  /** Creates the RDS RouteConfiguration. */
  @NotNull
  public RouteConfiguration build() {
    var routeBuilder =
        Route.newBuilder()
            .setMatch(RouteMatch.newBuilder().setPrefix(routePrefix).build())
            .setDecorator(Decorator.newBuilder().setOperation(decorator).build())
            .setNonForwardingAction(NonForwardingAction.getDefaultInstance());

    if (enableRbac) {
      routeBuilder.putTypedPerFilterConfig(
          HttpConnectionManagerBuilder.ENVOY_FILTER_HTTP_RBAC,
          Any.pack(createRbacPerRouteConfig("xds", "host-certs")));
    }

    var virtualHostBuilder =
        VirtualHost.newBuilder()
            .setName(virtualHostName)
            .addDomains("*")
            .addRoutes(routeBuilder.build());

    return RouteConfiguration.newBuilder()
        .setName(name)
        .addVirtualHosts(virtualHostBuilder.build())
        .build();
  }

  /**
   * Creates an RBACPerRoute config with a single policy called <code>greeter-clients</code>. The
   * policy applies to the base URL path of the <code>helloworld.Greeter</code> gRPC service, and it
   * permits workloads with an X.509 SVID for any Kubernetes ServiceAccount in the specified
   * Kubernetes Namespaces. If no allowed Namespaces are provided, this function defaults to
   * allowing all ServiceAccounts in all Namespaces.
   */
  @NotNull
  private static RBACPerRoute createRbacPerRouteConfig(String... allowNamespaces) {
    if (allowNamespaces == null || allowNamespaces.length == 0) {
      allowNamespaces = new String[] {".*"};
    }
    var pipedNamespaces = String.join("|", allowNamespaces);
    return RBACPerRoute.newBuilder()
        .setRbac(
            RBAC.newBuilder()
                .setRules(
                    // No alias imports in Java :-(
                    io.envoyproxy
                        .envoy
                        .config
                        .rbac
                        .v3
                        .RBAC
                        .newBuilder()
                        .setAction(io.envoyproxy.envoy.config.rbac.v3.RBAC.Action.ALLOW)
                        .putPolicies(
                            "greeter-clients",
                            Policy.newBuilder()
                                // Permissions can match URL path, headers/metadata, and more.
                                .addPermissions(
                                    Permission.newBuilder()
                                        .setUrlPath(
                                            PathMatcher.newBuilder()
                                                .setPath(
                                                    StringMatcher.newBuilder()
                                                        .setPrefix("/helloworld.Greeter/")
                                                        .build())
                                                .build())
                                        .build()
                                    // Permission.newBuilder().setAny(true).build()
                                    )
                                .addPrincipals(
                                    Principal.newBuilder()
                                        .setAuthenticated(
                                            Principal.Authenticated.newBuilder()
                                                .setPrincipalName(
                                                    // Matches against URI SANs, then DNS SANs, then
                                                    // Subject DN.
                                                    StringMatcher.newBuilder()
                                                        .setSafeRegex(
                                                            RegexMatcher.newBuilder()
                                                                .setRegex(
                                                                    "spiffe://[^/]+/ns/(%s)/sa/.+"
                                                                        .formatted(pipedNamespaces))
                                                                .build())
                                                        .build())
                                                .build())
                                        .build())
                                .build())
                        .build())
                .build())
        .build();
  }
}
