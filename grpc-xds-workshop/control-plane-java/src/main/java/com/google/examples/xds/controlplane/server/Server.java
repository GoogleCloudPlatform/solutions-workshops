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

package com.google.examples.xds.controlplane.server;

import com.google.examples.xds.controlplane.config.ServerConfig;
import com.google.examples.xds.controlplane.informers.InformerConfig;
import com.google.examples.xds.controlplane.informers.InformerManager;
import com.google.examples.xds.controlplane.interceptors.LoggingServerInterceptor;
import com.google.examples.xds.controlplane.xds.XdsSnapshotCache;
import io.envoyproxy.controlplane.cache.NodeGroup;
import io.envoyproxy.controlplane.server.V3DiscoveryServer;
import io.grpc.BindableService;
import io.grpc.Grpc;
import io.grpc.InsecureServerCredentials;
import io.grpc.ServerInterceptors;
import io.grpc.ServerServiceDefinition;
import io.grpc.health.v1.HealthCheckResponse.ServingStatus;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.protobuf.services.HealthStatusManager;
import io.grpc.protobuf.services.ProtoReflectionService;
import io.grpc.services.AdminInterface;
import java.io.IOException;
import java.util.Collection;
import java.util.concurrent.TimeUnit;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** The xDS control plane management server. */
public class Server {

  private static final Logger LOG = LoggerFactory.getLogger(Server.class);

  /** Fixed node hash, so all xDS clients access the same cache snapshot. */
  private static final NodeGroup<String> FIXED_HASH = node -> "default";

  /** Runs the server. */
  public void run(@NotNull ServerConfig config) throws Exception {
    var xdsCache = new XdsSnapshotCache<>(FIXED_HASH);
    InformerManager<String> informers = setupInformers(xdsCache, config.informers());
    informers.start();
    Runtime.getRuntime().addShutdownHook(new Thread(informers::stop));

    int controlPlanePort = config.servingPort();
    var discoveryServer = new V3DiscoveryServer(xdsCache);
    var health = new HealthStatusManager();
    health.setStatus("", ServingStatus.SERVING);
    var server = createManagementServer(controlPlanePort, discoveryServer, health);
    server.start();

    // Serve health, admin and reflection services on the health port
    var healthServer =
        Grpc.newServerBuilderForPort(config.healthPort(), InsecureServerCredentials.create())
            .addService(health.getHealthService())
            .addServices(AdminInterface.getStandardServices())
            .addService(ProtoReflectionService.newInstance())
            .build()
            .start();

    addServerShutdownHook(server, healthServer, health);
    LOG.info("xDS control plane management server listening on port {}", server.getPort());

    server.awaitTermination();
  }

  @NotNull
  private InformerManager<String> setupInformers(
      @NotNull XdsSnapshotCache<String> cache, @NotNull Collection<InformerConfig> informerConfigs)
      throws IOException {
    var informers = new InformerManager<>(cache);
    informerConfigs.forEach(
        conf -> informers.addEndpointSliceInformer(conf.namespace(), conf.services()));
    return informers;
  }

  @NotNull
  private io.grpc.Server createManagementServer(
      int port, V3DiscoveryServer discoveryServer, @NotNull HealthStatusManager health) {
    return NettyServerBuilder.forPort(port)
        .addService(withLogging(discoveryServer.getAggregatedDiscoveryServiceImpl()))
        .addService(withLogging(discoveryServer.getClusterDiscoveryServiceImpl()))
        .addService(withLogging(discoveryServer.getEndpointDiscoveryServiceImpl()))
        .addService(withLogging(discoveryServer.getListenerDiscoveryServiceImpl()))
        .addService(withLogging(discoveryServer.getRouteDiscoveryServiceImpl()))
        .addService(withLogging(discoveryServer.getSecretDiscoveryServiceImpl()))
        .addService(health.getHealthService())
        .addServices(AdminInterface.getStandardServices())
        .addService(ProtoReflectionService.newInstance())
        .build();
  }

  @NotNull
  private ServerServiceDefinition withLogging(@NotNull BindableService service) {
    return ServerInterceptors.intercept(service, new LoggingServerInterceptor());
  }

  private void addServerShutdownHook(
      io.grpc.Server server, io.grpc.Server healthServer, HealthStatusManager health) {
    Runtime.getRuntime()
        .addShutdownHook(
            new Thread(
                () -> {
                  // Mark all services as NOT_SERVING.
                  health.enterTerminalState();
                  // Start graceful shutdown
                  server.shutdown();
                  try {
                    // Wait for RPCs to complete processing
                    if (!server.awaitTermination(5, TimeUnit.SECONDS)) {
                      // That was plenty of time. Let's cancel the remaining RPCs.
                      server.shutdownNow();
                      // shutdownNow isn't instantaneous, so give a bit of time to clean resources
                      // up gracefully. Normally this will be well under a second.
                      server.awaitTermination(2, TimeUnit.SECONDS);
                    }
                    healthServer.shutdownNow();
                    healthServer.awaitTermination(2, TimeUnit.SECONDS);
                  } catch (InterruptedException ex) {
                    healthServer.shutdownNow();
                    server.shutdownNow();
                  }
                }));
  }
}
