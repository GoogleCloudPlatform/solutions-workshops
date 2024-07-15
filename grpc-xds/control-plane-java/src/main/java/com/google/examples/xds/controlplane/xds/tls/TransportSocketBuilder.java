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

package com.google.examples.xds.controlplane.xds.tls;

import com.google.protobuf.Any;
import com.google.protobuf.Message;
import io.envoyproxy.envoy.config.core.v3.TransportSocket;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext;
import org.jetbrains.annotations.NotNull;

/** Builds TransportSockets for upstream and downstream TLS contexts. */
public class TransportSocketBuilder {

  /**
   * TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME is used in the `[Down|Up]streamTlsContext`s.
   *
   * <p>Using the same name as the <a
   * href="https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/2a9cf4614b56ec085c391a12f4cc53defaa575ac/main.go#L276">
   * <code>traffic-director-grpc-bootstrap</code></a> tool, but this is not important.
   */
  public static final String TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME = "google_cloud_private_spiffe";

  /** Copied from {@link io.grpc.xds.XdsListenerResource}. */
  private static final String TRANSPORT_SOCKET_NAME_TLS = "envoy.transport_sockets.tls";

  private final Message upstreamOrDownstreamTlsContext;

  /** Provide either an UpstreamTlsContext or a DownstreamTlsContext. */
  public TransportSocketBuilder(@NotNull Message upstreamOrDownstreamTlsContext) {
    if (!(upstreamOrDownstreamTlsContext instanceof UpstreamTlsContext)
        && !(upstreamOrDownstreamTlsContext instanceof DownstreamTlsContext)) {
      throw new IllegalArgumentException(
          "Must provide either an UpstreamTlsContext or a DownstreamTlsContext");
    }
    this.upstreamOrDownstreamTlsContext = upstreamOrDownstreamTlsContext;
  }

  /** Creates the TransportSocket. */
  @NotNull
  public TransportSocket build() {
    return TransportSocket.newBuilder()
        .setName(TRANSPORT_SOCKET_NAME_TLS)
        .setTypedConfig(Any.pack(upstreamOrDownstreamTlsContext))
        .build();
  }
}
