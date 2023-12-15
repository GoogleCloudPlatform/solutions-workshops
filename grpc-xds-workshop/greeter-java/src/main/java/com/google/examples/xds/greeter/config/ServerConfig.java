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

package com.google.examples.xds.greeter.config;

import com.google.gson.JsonParser;
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.net.HttpURLConnection;
import java.net.InetAddress;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ThreadLocalRandom;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Reads greeter server configuration from the environment.
 *
 * <p>For simplified unit testing of the server, create a subclass and override methods.
 */
public class ServerConfig {

  private static final Logger LOG = LoggerFactory.getLogger(ServerConfig.class);
  private static final String DEFAULT_SERVING_PORT = "50051";
  private static final String DEFAULT_HEALTH_PORT = "50052";

  /** Returns the port number that the gRPC server should listen on for serving the greeter service. */
  public int servingPort() {
    return Integer.parseInt(System.getenv().getOrDefault("PORT", DEFAULT_SERVING_PORT));
  }

  /** Returns the port number that the gRPC server should listen on for health checks. */
  public int healthPort() {
    return Integer.parseInt(System.getenv().getOrDefault("HEALTH_PORT", DEFAULT_HEALTH_PORT));
  }

  /**
   * Returns the address of the next greeter in the chain, or empty string if this is a leaf
   * greeter.
   */
  public String nextHop() {
    return System.getenv().getOrDefault("NEXT_HOP", "");
  }

  /**
   * Determine if the server should be an xDS server.
   *
   * @see io.grpc.xds.BootstrapperImpl#BOOTSTRAP_PATH_SYS_ENV_VAR
   * @see io.grpc.xds.BootstrapperImpl#BOOTSTRAP_CONFIG_SYS_ENV_VAR
   * @see io.grpc.xds.BootstrapperImpl#BOOTSTRAP_PATH_SYS_PROPERTY
   * @see io.grpc.xds.BootstrapperImpl#BOOTSTRAP_CONFIG_SYS_PROPERTY
   */
  public boolean useXds() {
    return System.getenv("GRPC_XDS_BOOTSTRAP") != null
        || System.getenv("GRPC_XDS_BOOTSTRAP_CONFIG") != null
        || System.getProperty("io.grpc.xds.bootstrap") != null
        || System.getProperty("io.grpc.xds.bootstrapConfig") != null;
  }

  /** Create a greeter name from the host name and the zone name. */
  @NotNull
  public String greeterName() {
    String hostName = getHostName();
    String zone = getZoneFromGrpcXdsBootstrapFile();
    if (zone.isBlank()) {
      zone = getZoneFromGcpMetadataServer();
    }
    return hostName + "(" + zone + ")";
  }

  /**
   * Look up the host name.
   *
   * @return the host name, or a generated name if there is a problem looking up the host name
   */
  @NotNull
  String getHostName() {
    try {
      return InetAddress.getLocalHost().getHostName();
    } catch (IOException e) {
      LOG.warn("Could not determine host name. Using a generated name.", e);
    }
    // Should not happen, but if it does, generate a random name.
    return "node-" + ThreadLocalRandom.current().nextInt();
  }

  /**
   * Look up the zone name of the Kubernetes cluster node where this Pod is scheduled, by parsing
   * the locality information in the gRPC xDS bootstrap file or config.
   *
   * @return the zone name, or empty string if there is a problem looking up the zone
   */
  @NotNull
  String getZoneFromGrpcXdsBootstrapFile() {
    try {
      // Environment variables take precedence over system properties.
      String grpcXdsBootstrapFileName =
          System.getenv()
              .getOrDefault("GRPC_XDS_BOOTSTRAP", System.getProperty("io.grpc.xds.bootstrap"));
      String grpcXdsBootstrap =
          System.getenv()
              .getOrDefault(
                  "GRPC_XDS_BOOTSTRAP_CONFIG", System.getProperty("io.grpc.xds.bootstrapConfig"));
      if (grpcXdsBootstrapFileName == null && grpcXdsBootstrap == null) {
        // Not using xDS, so don't try parsing the gRPC xDS bootstrap config.
        return "";
      }
      String bootstrapJson = grpcXdsBootstrap;
      if (grpcXdsBootstrapFileName != null) {
        // GRPC_XDS_BOOTSTRAP takes precedence over GRPC_XDS_BOOTSTRAP_CONFIG
        bootstrapJson = Files.readString(Path.of(grpcXdsBootstrapFileName), StandardCharsets.UTF_8);
      }
      return JsonParser.parseString(bootstrapJson)
          .getAsJsonObject()
          .getAsJsonObject("node")
          .getAsJsonObject("locality")
          .get("zone")
          .getAsString();
    } catch (IOException e) {
      LOG.warn("Could not determine the zone from the gRPC xDS bootstrap configuration.", e);
    }
    return "";
  }

  /**
   * Look up the zone name of the Kubernetes cluster node where this Pod is scheduled, by querying
   * the Google Kubernetes Engine or Compute Engine <a
   * href="https://cloud.google.com/compute/docs/metadata/overview">metadata server</a>.
   *
   * @return the zone name, or empty string if there is a problem looking up the zone
   */
  @NotNull
  String getZoneFromGcpMetadataServer() {
    try {
      var url = new URL("http://metadata.google.internal/computeMetadata/v1/instance/zone");
      var connection = (HttpURLConnection) url.openConnection();
      connection.setRequestMethod("GET");
      connection.setRequestProperty("Metadata-Flavor", "Google");
      connection.setConnectTimeout(500);
      connection.setReadTimeout(500);
      int responseCode = connection.getResponseCode();
      if (responseCode != 200) {
        LOG.warn(
            "Could not look up the zone from the GCP metadata server, got response code {}.",
            responseCode);
        return "";
      }
      try (var responseBodyReader =
          new BufferedReader(
              new InputStreamReader(connection.getInputStream(), StandardCharsets.ISO_8859_1))) {
        String firstLineOfResponse = responseBodyReader.readLine();
        if (firstLineOfResponse == null) {
          LOG.warn(
              "Could not look up the zone from the GCP metadata server, got empty response body.");
          return "";
        }
        // Response is <code>projects/[PROJECT_NUMBER]/zones/[ZONE]</code>.
        String[] split = firstLineOfResponse.split("/");
        if (split.length < 4) {
          LOG.warn(
              "Unexpected zone format from the GCP metadata server: {}, using it verbatim.",
              firstLineOfResponse);
          return firstLineOfResponse;
        }
        return split[3];
      }
    } catch (IOException e) {
      LOG.warn("Could not look up the zone from the GCP metadata server: {}", e.getMessage());
    }
    return "";
  }
}
