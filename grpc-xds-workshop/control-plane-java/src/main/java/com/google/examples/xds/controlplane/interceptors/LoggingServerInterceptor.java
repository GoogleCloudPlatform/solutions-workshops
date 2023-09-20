// Copyright 2023 Google LLC
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

package com.google.examples.xds.controlplane.interceptors;

import com.google.examples.xds.controlplane.logging.MessagePrinter;
import io.envoyproxy.envoy.config.cluster.v3.Cluster;
import io.envoyproxy.envoy.config.endpoint.v3.ClusterLoadAssignment;
import io.envoyproxy.envoy.config.route.v3.RouteConfiguration;
import io.envoyproxy.envoy.config.route.v3.VirtualHost;
import io.envoyproxy.envoy.extensions.clusters.aggregate.v3.ClusterConfig;
import io.envoyproxy.envoy.extensions.filters.http.fault.v3.HTTPFault;
import io.envoyproxy.envoy.extensions.filters.http.rbac.v3.RBAC;
import io.envoyproxy.envoy.extensions.filters.http.rbac.v3.RBACPerRoute;
import io.envoyproxy.envoy.extensions.filters.http.router.v3.Router;
import io.envoyproxy.envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.Secret;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext;
import io.envoyproxy.envoy.service.discovery.v3.DiscoveryRequest;
import io.envoyproxy.envoy.service.discovery.v3.DiscoveryResponse;
import io.grpc.ForwardingServerCall.SimpleForwardingServerCall;
import io.grpc.ForwardingServerCallListener.SimpleForwardingServerCallListener;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCall.Listener;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import io.grpc.Status.Code;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Logs requests received and responses sent by a server. */
public class LoggingServerInterceptor implements ServerInterceptor {

  private static final Logger LOG = LoggerFactory.getLogger(LoggingServerInterceptor.class);

  private final MessagePrinter messagePrinter;

  /**
   * Create an interceptor configured to log DiscoveryRequests, DiscoveryResponses, and other xDS
   * resources as JSON.
   */
  public LoggingServerInterceptor() {
    this.messagePrinter =
        new MessagePrinter(
            DiscoveryRequest.getDescriptor(),
            DiscoveryResponse.getDescriptor(),
            io.envoyproxy.envoy.config.listener.v3.Listener.getDescriptor(),
            HttpConnectionManager.getDescriptor(),
            HTTPFault.getDescriptor(),
            Router.getDescriptor(),
            RBAC.getDescriptor(),
            RBACPerRoute.getDescriptor(),
            UpstreamTlsContext.getDescriptor(),
            DownstreamTlsContext.getDescriptor(),
            RouteConfiguration.getDescriptor(),
            VirtualHost.getDescriptor(),
            Cluster.getDescriptor(),
            ClusterConfig.getDescriptor(),
            ClusterLoadAssignment.getDescriptor(),
            Secret.getDescriptor());
  }

  @Override
  public <ReqT, RespT> Listener<ReqT> interceptCall(
      ServerCall<ReqT, RespT> call, Metadata requestHeaders, ServerCallHandler<ReqT, RespT> next) {
    return new SimpleForwardingServerCallListener<>(
        next.startCall(
            new SimpleForwardingServerCall<ReqT, RespT>(call) {
              @Override
              public void sendMessage(RespT message) {
                if (LOG.isDebugEnabled()) {
                  LOG.debug(
                      "Sending response {}:\n{}",
                      message.getClass().getCanonicalName(),
                      messagePrinter.print(message));
                }
                super.sendMessage(message);
              }

              @Override
              public void close(Status status, Metadata trailers) {
                if (!status.isOk() && status.getCode() != Code.CANCELLED) {
                  LOG.error(
                      "RPC status code: {} description: {}",
                      status.getCode(),
                      status.getDescription(),
                      status.asException(trailers));
                } else {
                  LOG.debug(
                      "RPC status code: {} description: {}",
                      status.getCode(),
                      status.getDescription());
                }
                super.close(status, trailers);
              }
            },
            requestHeaders)) {
      @Override
      public void onMessage(ReqT message) {
        if (LOG.isDebugEnabled()) {
          LOG.debug(
              "Received request {}:\n{}",
              message.getClass().getCanonicalName(),
              messagePrinter.print(message));
        }
        super.onMessage(message);
      }
    };
  }
}
