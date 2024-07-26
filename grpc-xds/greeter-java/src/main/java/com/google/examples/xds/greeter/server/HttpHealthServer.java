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

package com.google.examples.xds.greeter.server;

import io.grpc.health.v1.HealthCheckRequest;
import io.grpc.health.v1.HealthCheckResponse;
import io.grpc.health.v1.HealthCheckResponse.ServingStatus;
import io.grpc.health.v1.HealthGrpc;
import io.grpc.health.v1.HealthGrpc.HealthImplBase;
import io.grpc.protobuf.services.HealthStatusManager;
import io.grpc.stub.StreamObserver;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import org.apache.http.client.utils.URLEncodedUtils;
import org.apache.http.message.BasicNameValuePair;
import org.eclipse.jetty.http.HttpHeader;
import org.eclipse.jetty.http.HttpStatus;
import org.eclipse.jetty.http2.server.HTTP2CServerConnectionFactory;
import org.eclipse.jetty.server.CustomRequestLog;
import org.eclipse.jetty.server.Handler;
import org.eclipse.jetty.server.HttpConnectionFactory;
import org.eclipse.jetty.server.Request;
import org.eclipse.jetty.server.Response;
import org.eclipse.jetty.server.ServerConnector;
import org.eclipse.jetty.server.Slf4jRequestLogWriter;
import org.eclipse.jetty.util.BufferUtil;
import org.eclipse.jetty.util.Callback;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * HTTP and h2c (plaintext HTTP/2) health check listener.
 *
 * <p>Accepts an optional URL query parameter named &quot;service&quot;. The (first) value of this
 * query parameter is the name of the gRPC service to be health checked, e.g.
 * &quot;helloworld.Greeter&quot;. If absent, the empty string is used as the gRPC service name.
 */
public class HttpHealthServer {

  private static final Logger LOG = LoggerFactory.getLogger(Server.class);

  private final org.eclipse.jetty.server.Server server;

  public HttpHealthServer(int port, String urlPath, HealthStatusManager healthStatusManager) {
    this.server = new org.eclipse.jetty.server.Server();
    var serverConnector =
        new ServerConnector(
            server, new HttpConnectionFactory(), new HTTP2CServerConnectionFactory());
    serverConnector.setPort(port);
    server.addConnector(serverConnector);
    server.setStopAtShutdown(true);
    var requestLogWriter = new Slf4jRequestLogWriter();
    requestLogWriter.setLoggerName(HttpHealthServer.class.getName());
    server.setRequestLog(
        new CustomRequestLog(requestLogWriter, CustomRequestLog.EXTENDED_NCSA_FORMAT));
    var healthGrpc = (HealthGrpc.HealthImplBase) healthStatusManager.getHealthService();
    server.setHandler(new HealthCheckHandler(urlPath, healthGrpc));
    LOG.info("Listening for http/h2c health checks on port {}", port);
  }

  public void start() {
    try {
      this.server.start();
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  private static class HealthCheckHandler extends Handler.Abstract {

    private final String urlPath;
    private final HealthImplBase healthGrpc;

    public HealthCheckHandler(String urlPath, HealthImplBase healthGrpc) {
      this.urlPath = urlPath;
      this.healthGrpc = healthGrpc;
    }

    @Override
    public boolean handle(Request request, Response response, Callback callback) throws Exception {
      if (request == null
          || request.getHttpURI() == null
          || !urlPath.equals(request.getHttpURI().getPath())) {
        return false;
      }
      var latch = new CountDownLatch(1);
      String grpcService =
          URLEncodedUtils.parse(request.getHttpURI().toURI(), StandardCharsets.UTF_8).stream()
              .filter(nameValuePair -> nameValuePair.getName().equalsIgnoreCase("service"))
              .findFirst()
              .orElse(new BasicNameValuePair("service", ""))
              .getValue();
      response.getHeaders().put(HttpHeader.CONTENT_TYPE, "text/plain; charset=utf-8");
      healthGrpc.check(
          HealthCheckRequest.newBuilder().setService(grpcService).build(),
          new StreamObserver<>() {
            @Override
            public void onNext(HealthCheckResponse healthResp) {
              ServingStatus servingStatus = healthResp.getStatus();
              if (servingStatus == ServingStatus.SERVING) {
                response.setStatus(HttpStatus.OK_200);
                response.write(
                    true,
                    BufferUtil.toBuffer(servingStatus.name(), StandardCharsets.UTF_8),
                    callback);
              } else {
                response.setStatus(HttpStatus.INTERNAL_SERVER_ERROR_500);
                response.write(
                    true,
                    BufferUtil.toBuffer(servingStatus.name(), StandardCharsets.UTF_8),
                    callback);
              }
              latch.countDown();
            }

            @Override
            public void onError(Throwable t) {
              response.setStatus(HttpStatus.INTERNAL_SERVER_ERROR_500);
              response.write(
                  true, BufferUtil.toBuffer(t.getMessage(), StandardCharsets.UTF_8), callback);
              LOG.error("HTTP health check onError:", t);
              latch.countDown();
            }

            @Override
            public void onCompleted() {}
          });
      return latch.await(3, TimeUnit.SECONDS);
    }
  }
}
