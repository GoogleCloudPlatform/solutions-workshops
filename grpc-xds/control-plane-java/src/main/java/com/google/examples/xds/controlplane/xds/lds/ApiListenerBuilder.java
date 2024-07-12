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

import com.google.examples.xds.controlplane.xds.lds.HttpConnectionManagerBuilder.ForApiListener;
import com.google.protobuf.Any;
import io.envoyproxy.envoy.config.listener.v3.ApiListener;
import io.envoyproxy.envoy.config.listener.v3.Listener;
import org.jetbrains.annotations.NotNull;

/**
 * Create an LDS Listener of type <a
 * href="https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/listener/v3/api_listener.proto">API
 * listener</a>.
 *
 * <p>API listeners are used by xDS-enabled gRPC clients (<a
 * href="https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#listener-proto">gRFC
 * A27: xDS-Based Global Load Balancing</a>) for upstream connections to gRPC servers.
 */
public class ApiListenerBuilder {
  private final String name;
  private String statPrefix;
  private String routeConfigurationName;

  /**
   * Builder for LDS API Listeners.
   *
   * @param name the Listener name, also used for HttpConnectionManagerBuilder statPrefix and RDS
   *     routeConfigurationName, unless overridden.
   */
  public ApiListenerBuilder(@NotNull String name) {
    if (name.isBlank()) {
      throw new IllegalArgumentException("Listener name must not be blank");
    }
    this.name = name;
    this.statPrefix = name;
    this.routeConfigurationName = name;
  }

  /** Human-readable prefix used when emitting statistics. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ApiListenerBuilder withStatPrefix(@NotNull String statPrefix) {
    if (statPrefix.isBlank()) {
      throw new IllegalArgumentException("Listener statPrefix must not be blank");
    }
    this.statPrefix = statPrefix;
    return this;
  }

  /**
   * Sets the RDS RouteConfiguration name used by the Listener.
   *
   * @param routeConfigurationName the name of the RDS route configuration referenced by this API
   *     listener.
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ApiListenerBuilder withRouteConfigurationName(@NotNull String routeConfigurationName) {
    if (routeConfigurationName.isBlank()) {
      throw new IllegalArgumentException("Listener routeConfigurationName must not be blank");
    }
    this.routeConfigurationName = routeConfigurationName;
    return this;
  }

  /**
   * Builds the API Listener.
   *
   * @return an LDS API Listener for a gRPC client.
   */
  @NotNull
  public Listener build() {
    var httpConnectionManager =
        new ForApiListener(routeConfigurationName)
            .withStatPrefix(statPrefix)
            .build();
    return Listener.newBuilder()
        .setName(name)
        .setApiListener(
            ApiListener.newBuilder().setApiListener(Any.pack(httpConnectionManager)).build())
        .build();
  }
}
