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

import com.google.examples.xds.controlplane.xds.lds.HttpConnectionManagerBuilder.ForSocketListener;
import io.envoyproxy.envoy.config.listener.v3.Listener;
import org.jetbrains.annotations.NotNull;

/**
 * Creates an LDS socket Listener to be used by Envoy proxy instances that fetch dynamic
 * configuration using xDS.
 *
 * <p>The Listener does not require clients to present X.509 certificates for mTLS.
 *
 * <p>TODO: Add gRPC-JSON transcoding and gRPC HTTP/1.1 bridge.
 *
 * @see <a
 *     href="https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_json_transcoder_filter">gRPC-JSON
 *     transcoder</a>
 * @see <a
 *     href="https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_http1_bridge_filter">gRPC
 *     HTTP/1.1 bridge</a>
 * @see <a
 *     href="https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_http1_reverse_bridge_filter">gRPC
 *     HTTP/1.1 reverse bridge</a>
 */
public class EnvoyListenerBuilder {
  private static final String ENVOY_LISTENER_NAME_PREFIX = "envoy-listener";
  public static final String ENVOY_ROUTE_CONFIGURATION_NAME = "envoy-route-configuration";

  private final int port;
  private String socketAddress;
  private boolean enableTls;

  public EnvoyListenerBuilder(int port) {
    this.port = port;
    this.socketAddress = "0.0.0.0"; // use "::" for an IPv6 listener with IPv4 compatibility
  }

  /**
   * Socket address is typically either <code>"0.0.0.0"</code> (for IPv4) or <code>"::"</code> for
   * IPv6.
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public EnvoyListenerBuilder withSocketAddress(@NotNull String socketAddress) {
    if (socketAddress.isBlank()) {
      throw new IllegalArgumentException("EnvoyListener socketAddress must be non-blank");
    }
    this.socketAddress = socketAddress;
    return this;
  }

  /** Determines if the Envoy proxy presents a server TLS certificate to downstream clients. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public EnvoyListenerBuilder withEnableTls(boolean enableTls) {
    this.enableTls = enableTls;
    return this;
  }

  /** Builds the socket Listener for Envoy proxies. */
  @NotNull
  public Listener build() {
    String listenerName = "%s-%d".formatted(ENVOY_LISTENER_NAME_PREFIX, port);
    var httpConnectionManager =
        new ForSocketListener(ENVOY_ROUTE_CONFIGURATION_NAME)
            .withStatPrefix(listenerName)
            .build();

    return new SocketListenerBuilder(listenerName, port, httpConnectionManager)
        .withSocketAddress(socketAddress)
        .withEnableTls(enableTls)
        .withRequireClientCerts(false)
        .build();
  }
}
