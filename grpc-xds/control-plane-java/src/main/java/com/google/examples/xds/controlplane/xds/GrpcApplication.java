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

package com.google.examples.xds.controlplane.xds;

import java.util.Collection;
import java.util.List;
import org.jetbrains.annotations.NotNull;

/** Represents a gRPC application that clients discover using xDS. */
public record GrpcApplication(
    @NotNull String namespace,
    @NotNull String serviceAccountName,
    @NotNull String listenerName,
    @NotNull String routeName,
    @NotNull String clusterName,
    @NotNull String pathPrefix,
    int port,
    @NotNull Collection<GrpcApplicationEndpoint> endpoints) {

  /**
   * Convenience constructor for use in the common scenario where the Kubernetes ServiceAccount,
   * Listener, RouteConfiguration, and Cluster all use the same name.
   *
   * @param namespace the Kubernetes Namespace
   * @param name the Kubernetes ServiceAccount, LDS Listener, RDS RouteConfiguration, and CDS
   *     Cluster name
   * @param port listening port of the server
   * @param endpoints application server endpoints
   */
  public GrpcApplication(
      @NotNull String namespace,
      @NotNull String name,
      int port,
      @NotNull Collection<GrpcApplicationEndpoint> endpoints) {
    this(namespace, name, name, name, name, "", port, endpoints);
  }

  /**
   * Canonical constructor.
   *
   * @param namespace the Kubernetes Namespace where the application is running
   * @param serviceAccountName the name of the Kubernetes Service Account used by the application
   * @param listenerName the name that clients should use to connect to the application, following
   *     the <code>xds:///</code> scheme prefix. For instance, if <code>listenerName</code> is
   *     <code>greeter-leaf</code>, clients connect using the {@link io.grpc.NameResolver}-compliant
   *     URI <code>xds:///greeter-leaf</code>
   * @param routeName can be any value, but it is typically the same as <code>listenerName</code>
   * @param clusterName can be any value, but it is typically the same as <code>listenerName</code>
   * @param pathPrefix URL path prefix for the service exposed by the application. For a single
   *     route, the path prefix can be an empty string. In general, for a gRPC service, the path
   *     prefix will be in the format <code>/[package].[service]/</code>, e.g., <code>
   *     /helloworld.Greeter/</code>
   * @param port listening port of the server
   * @param endpoints application server endpoints
   */
  public GrpcApplication(
      @NotNull String namespace,
      @NotNull String serviceAccountName,
      @NotNull String listenerName,
      @NotNull String routeName,
      @NotNull String clusterName,
      @NotNull String pathPrefix,
      int port,
      @NotNull Collection<GrpcApplicationEndpoint> endpoints) {
    this.namespace = namespace;
    this.serviceAccountName = serviceAccountName;
    this.listenerName = listenerName;
    this.routeName = routeName;
    this.clusterName = clusterName;
    this.pathPrefix = pathPrefix;
    this.port = port;
    this.endpoints = List.copyOf(endpoints);
  }
}
