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

import com.google.examples.xds.controlplane.xds.tls.DownstreamTlsContextBuilder;
import com.google.examples.xds.controlplane.xds.tls.TransportSocketBuilder;
import com.google.protobuf.Any;
import com.google.protobuf.BoolValue;
import io.envoyproxy.envoy.config.core.v3.Address;
import io.envoyproxy.envoy.config.core.v3.SocketAddress;
import io.envoyproxy.envoy.config.core.v3.SocketAddress.Protocol;
import io.envoyproxy.envoy.config.core.v3.TrafficDirection;
import io.envoyproxy.envoy.config.listener.v3.Filter;
import io.envoyproxy.envoy.config.listener.v3.FilterChain;
import io.envoyproxy.envoy.config.listener.v3.Listener;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager;
import java.net.Inet6Address;
import java.net.InetAddress;
import java.net.UnknownHostException;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Builds LDS socket Listener resources for gRPC servers and Envoy proxy instances. */
public class SocketListenerBuilder {

  private static final Logger LOG = LoggerFactory.getLogger(SocketListenerBuilder.class);

  /** Copied from {@link io.envoyproxy.controlplane.cache.Resources}. */
  private static final String ENVOY_HTTP_CONNECTION_MANAGER = "envoy.http_connection_manager";

  private final String name;
  private final int port;
  private final HttpConnectionManager httpConnectionManager;
  private String socketAddress = "0.0.0.0";
  private boolean enableTls;
  private boolean requireClientCerts;

  /** Name will be the listener name. */
  public SocketListenerBuilder(
      @NotNull String name, int port, @NotNull HttpConnectionManager httpConnectionManager) {
    if (name.isBlank()) {
      throw new IllegalArgumentException("SocketListener name must be non-blank");
    }
    this.name = name;
    this.port = port;
    this.httpConnectionManager = httpConnectionManager;
  }

  /**
   * Socket address is typically either <code>"0.0.0.0"</code> (for IPv4) or <code>"::"</code> for
   * IPv6.
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public SocketListenerBuilder withSocketAddress(@NotNull String socketAddress) {
    if (socketAddress.isBlank()) {
      throw new IllegalArgumentException("Listener socketAddress must be non-blank");
    }
    this.socketAddress = socketAddress;
    return this;
  }

  /** Determines if the server presents a TLS certificate to downstream clients. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public SocketListenerBuilder withEnableTls(boolean enableTls) {
    this.enableTls = enableTls;
    return this;
  }

  /** Sets {@link #enableTls} to <code>true</code> as well. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public SocketListenerBuilder withRequireClientCerts(boolean requireClientCerts) {
    this.enableTls = true;
    this.requireClientCerts = requireClientCerts;
    return this;
  }

  @NotNull
  public Listener build() {
    return createSocketListener(
        name, socketAddress, port, httpConnectionManager, enableTls, requireClientCerts);
  }

  /** Builds the LDS socket Listener. */
  @NotNull
  public static Listener createSocketListener(
      @NotNull String listenerName,
      @NotNull String socketAddress,
      int port,
      @NotNull HttpConnectionManager httpConnectionManager,
      boolean enableTls,
      boolean requireClientCerts) {
    var filterChainBuilder =
        FilterChain.newBuilder()
            .addFilters(
                Filter.newBuilder()
                    .setName(ENVOY_HTTP_CONNECTION_MANAGER) // must be the last filter
                    .setTypedConfig(Any.pack(httpConnectionManager))
                    .build());

    if (enableTls) {
      filterChainBuilder.setTransportSocket(
          new TransportSocketBuilder(
                  new DownstreamTlsContextBuilder()
                      .withRequireClientCerts(requireClientCerts)
                      .build())
              .build());
    }

    boolean enableIpv4Compat = false;
    try {
      if (InetAddress.getByName(socketAddress) instanceof Inet6Address) {
        enableIpv4Compat = true;
      }
    } catch (UnknownHostException e) {
      LOG.warn("Could not determine IPv4 or IPv6 of socketAddress={}", socketAddress, e);
    }
    var addressBuilder =
        Address.newBuilder()
            .setSocketAddress(
                SocketAddress.newBuilder()
                    .setAddress(socketAddress)
                    .setPortValue(port)
                    .setProtocol(Protocol.TCP)
                    .setIpv4Compat(enableIpv4Compat)
                    .build());

    return Listener.newBuilder()
        .setName(listenerName)
        .setAddress(addressBuilder.build())
        .addFilterChains(filterChainBuilder.build())
        .setTrafficDirection(TrafficDirection.INBOUND)
        .setEnableReusePort(BoolValue.of(true))
        .build();
  }
}
