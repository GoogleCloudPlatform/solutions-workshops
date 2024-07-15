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

package com.google.examples.xds.controlplane.applications;

import io.envoyproxy.envoy.config.core.v3.HealthStatus;
import io.kubernetes.client.openapi.models.V1EndpointConditions;
import javax.annotation.Nullable;
import org.jetbrains.annotations.NotNull;

/**
 * Represents the serving status of an endpoint.
 *
 * <p>Translates from Kubernetes EndpointSlice conditions to xDS HealthStatus as used in EDS
 * ClusterLoadAssignment resources.
 */
public enum EndpointStatus {
  HEALTHY(HealthStatus.HEALTHY),
  UNHEALTHY(HealthStatus.UNHEALTHY),
  DRAINING(HealthStatus.DRAINING);

  private final HealthStatus healthStatus;

  EndpointStatus(HealthStatus healthStatus) {
    this.healthStatus = healthStatus;
  }

  /** Determines EndpointStatus from EndpointSlice {@code conditions}. */
  @NotNull
  public static EndpointStatus fromConditions(@Nullable V1EndpointConditions endpointConditions) {
    if (endpointConditions == null || Boolean.FALSE.equals(endpointConditions.getServing())) {
      return UNHEALTHY;
    }
    if (Boolean.TRUE.equals(endpointConditions.getTerminating())) {
      return DRAINING;
    }
    if (Boolean.TRUE.equals(endpointConditions.getReady())
        && Boolean.TRUE.equals(endpointConditions.getServing())) {
      // https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#ready
      return HEALTHY;
    }
    return UNHEALTHY;
  }

  @NotNull
  public HealthStatus getHealthStatus() {
    return healthStatus;
  }
}
