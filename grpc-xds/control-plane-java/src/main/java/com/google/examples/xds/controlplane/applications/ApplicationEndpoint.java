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

/** Represents an application endpoint, e.g, the endpoint of an xDS-enabled gRPC server. */
public record ApplicationEndpoint(
    @NotNull String node,
    @NotNull String zone,
    @NotNull Collection<String> addresses,
    EndpointStatus endpointStatus) {

  /**
   * Represents an application endpoint.
   *
   * @param node VM name
   * @param zone cloud provider zone name
   * @param addresses is typically just a list of one entry, but since the Kubernetes API spec for
   *     EndpointSlices allows for multiple addresses, this class uses a collection.
   * @param endpointStatus health status of the endpoint
   */
  public ApplicationEndpoint(
      @NotNull String node,
      @NotNull String zone,
      @NotNull Collection<String> addresses,
      @NotNull EndpointStatus endpointStatus) {
    this.node = node;
    this.zone = zone;
    this.addresses = List.copyOf(addresses);
    this.endpointStatus = endpointStatus;
  }
}
