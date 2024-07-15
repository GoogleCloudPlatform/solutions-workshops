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

import com.google.examples.xds.controlplane.xds.XdsSnapshotCache;
import com.google.examples.xds.controlplane.xds.lds.HttpConnectionManagerBuilder.ForSocketListener;
import com.google.examples.xds.controlplane.xds.rds.RouteConfigForGrpcServerBuilder;
import io.envoyproxy.envoy.config.listener.v3.Listener;
import org.jetbrains.annotations.NotNull;

/** Creates an LDS Listener to be used by xDS-enabled gRPC servers for downstream connections. */
public class GrpcServerListenerBuilder {
  private final String address;
  private final int port;
  private boolean enableTls;
  private boolean requireClientCerts;
  private boolean enableRbac;

  /** All gRPC server Listeners need a socket address with port. */
  public GrpcServerListenerBuilder(@NotNull String address, int port) {
    if (address.isBlank()) {
      throw new IllegalArgumentException("Listener address must not be blank");
    }
    this.address = address;
    this.port = port;
  }

  /** Enables TLS, use {@link #withRequireClientCerts(boolean)}} in addition to enable mTLS. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public GrpcServerListenerBuilder withEnableTls(boolean enableTls) {
    this.enableTls = enableTls;
    return this;
  }

  /** Sets {@link #enableTls} to <code>true</code> as well. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public GrpcServerListenerBuilder withRequireClientCerts(boolean requireClientCerts) {
    this.enableTls = true;
    this.requireClientCerts = requireClientCerts;
    return this;
  }

  /** RBAC here is client workload authorization. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public GrpcServerListenerBuilder withEnableRbac(boolean enableRbac) {
    this.enableRbac = enableRbac;
    return this;
  }

  /**
   * Create the server listener for xDS-enabled gRPC servers.
   *
   * @return a Listener, using RDS for the RouteConfiguration
   */
  @NotNull
  public Listener build() {
    String listenerName =
        XdsSnapshotCache.GRPC_SERVER_LISTENER_RESOURCE_NAME_TEMPLATE.formatted(
            address + ":" + port);

    var socketAddress = address;
    if (address.startsWith("[") && address.endsWith("]")) {
      // Special IPv6 address handling ("[::]" -> "::"):
      socketAddress = address.substring(1, address.length() - 1);
    }

    var httpConnectionManager =
        new ForSocketListener(
                RouteConfigForGrpcServerBuilder.ROUTE_CONFIGURATION_NAME_FOR_GRPC_SERVER_LISTENER)
            .withEnableRbac(enableRbac)
            .withStatPrefix(listenerName)
            .build();

    return new SocketListenerBuilder(listenerName, port, httpConnectionManager)
        .withSocketAddress(socketAddress)
        .withEnableTls(enableTls)
        .withRequireClientCerts(requireClientCerts)
        .build();
  }
}
