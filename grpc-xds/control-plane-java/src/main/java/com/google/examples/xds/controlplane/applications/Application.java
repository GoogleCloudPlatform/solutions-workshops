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

package com.google.examples.xds.controlplane.applications;

import java.util.Collection;
import java.util.List;
import org.jetbrains.annotations.NotNull;

/**
 * Represents an application, e.g., a gRPC server, that clients discover using xDS.
 *
 * @param namespace the Kubernetes Namespace where the application is running
 * @param serviceAccountName the name of the Kubernetes Service Account used by the application
 * @param name the name that clients should use to connect to the application, following the <code>
 *     xds:///</code> scheme prefix. For instance, if <code>name</code> is <code>
 *     greeter-leaf</code>, clients connect using the {@link io.grpc.NameResolver}-compliant URI
 *     <code>xds:///greeter-leaf</code>
 * @param pathPrefix URL path prefix for the service exposed by the application. For a single route,
 *     the path prefix can be an empty string. In general, for a gRPC service, the path prefix will
 *     be in the format <code>/[package].[service]/</code>, e.g., <code>
 *     /helloworld.Greeter/</code>
 * @param servingPort serving port of the server
 * @param healthCheckPort health check port of the server, can be the same as the serving port
 * @param endpoints application server endpoints
 */
public record Application(
    @NotNull String namespace,
    @NotNull String serviceAccountName,
    @NotNull String name,
    @NotNull String pathPrefix,
    int servingPort,
    @NotNull String servingProtocol,
    int healthCheckPort,
    @NotNull String healthCheckProtocol,
    @NotNull Collection<ApplicationEndpoint> endpoints) {

  /**
   * Convenience constructor for use in the common scenario where the Kubernetes ServiceAccount and
   * application share the same name.
   *
   * @param namespace the Kubernetes Namespace
   * @param name the Kubernetes ServiceAccount name and application name - which must match to use
   *     this constructor!
   * @param servingPort serving port of the server
   * @param healthCheckPort health check port of the server, can be the same as the serving port
   * @param endpoints application server endpoints
   */
  public Application(
      @NotNull String namespace,
      @NotNull String name,
      int servingPort,
      @NotNull String servingProtocol,
      int healthCheckPort,
      @NotNull String healthCheckProtocol,
      @NotNull Collection<ApplicationEndpoint> endpoints) {
    this(
        namespace,
        name,
        name,
        "",
        servingPort,
        servingProtocol,
        healthCheckPort,
        healthCheckProtocol,
        List.copyOf(endpoints));
  }
}
