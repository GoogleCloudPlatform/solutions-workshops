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

package com.google.examples.xds.controlplane.xds.eds;

import com.google.examples.xds.controlplane.applications.ApplicationEndpoint;
import com.google.examples.xds.controlplane.applications.EndpointStatus;
import com.google.protobuf.UInt32Value;
import io.envoyproxy.envoy.config.core.v3.Address;
import io.envoyproxy.envoy.config.core.v3.Locality;
import io.envoyproxy.envoy.config.core.v3.SocketAddress;
import io.envoyproxy.envoy.config.endpoint.v3.ClusterLoadAssignment;
import io.envoyproxy.envoy.config.endpoint.v3.ClusterLoadAssignment.Policy;
import io.envoyproxy.envoy.config.endpoint.v3.Endpoint;
import io.envoyproxy.envoy.config.endpoint.v3.LbEndpoint;
import io.envoyproxy.envoy.config.endpoint.v3.LocalityLbEndpoints;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Builds an EDS ClusterLoadAssignment resource. */
public class ClusterLoadAssignmentBuilder<T> {
  private static final Logger LOG = LoggerFactory.getLogger(ClusterLoadAssignmentBuilder.class);

  private final Set<ApplicationEndpoint> endpoints;
  private final int servingPort;
  private String edsServiceName;
  private T nodeHash;
  private LocalityPriorityMapper<T> localityPriorityMapper;

  /**
   * Provide all the endpoints for the ClusterLoadAssignment resource.
   */
  public ClusterLoadAssignmentBuilder(
      @NotNull Collection<ApplicationEndpoint> endpoints, int servingPort) {
    this.endpoints = new HashSet<>(endpoints);
    this.servingPort = servingPort;
  }

  /** EDS ServiceName is mandatory when using names with the xdstp:// scheme for xDS federation. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ClusterLoadAssignmentBuilder<T> withEdsServiceName(@NotNull String edsServiceName) {
    if (edsServiceName.isBlank()) {
      throw new IllegalArgumentException("ClusterLoadAssignment edsServiceName must be non-blank");
    }
    this.edsServiceName = edsServiceName;
    return this;
  }

  /** How to map localities to priorities. */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public ClusterLoadAssignmentBuilder<T> withLocalityPriorityMapper(
      @NotNull T nodeHash, @NotNull LocalityPriorityMapper<T> localityPriorityMapper) {
    this.nodeHash = nodeHash;
    this.localityPriorityMapper = localityPriorityMapper;
    return this;
  }

  /** Creates the EDS ClusterLoadAssignment resource. */
  @NotNull
  public ClusterLoadAssignment build() {
    var clusterLoadAssignmentBuilder = ClusterLoadAssignment.newBuilder();
    if (this.edsServiceName != null) {
      clusterLoadAssignmentBuilder.setClusterName(this.edsServiceName);
    }
    Map<Locality, List<ApplicationEndpoint>> endpointsByLocality =
        this.endpoints.stream()
            .collect(
                Collectors.groupingBy(
                    endpoint -> Locality.newBuilder().setZone(endpoint.zone()).build()));
    Map<Locality, Integer> localitiesByPriority =
        this.localityPriorityMapper.buildPriorityMap(this.nodeHash, endpointsByLocality.keySet());
    for (var locality : endpointsByLocality.keySet()) {
      int priority = localitiesByPriority.getOrDefault(locality, 0);
      LOG.debug(
          "Using priority={} for endpointZone={} and nodeHash={}",
          priority,
          locality.getZone(),
          this.nodeHash);
      var localityLbEndpointsBuilder =
          LocalityLbEndpoints.newBuilder()
              // Locality must be unique for a given priority.
              .setLocality(locality)
              // Weight is effectively mandatory, read the javadoc carefully :-)
              // Use number of endpoints in locality as weight, so assume all endpoints can handle
              // the same load.
              .setLoadBalancingWeight(UInt32Value.of(endpointsByLocality.get(locality).size()))
              // Priority is optional and defaults to 0. If provided, must start from 0 and have no
              // gaps.
              // Priority 0 is the highest priority.
              .setPriority(priority);
      Map<String, EndpointStatus> addressesForLocality = new HashMap<>();
      for (var endpoint : endpointsByLocality.get(locality)) {
        for (String address : endpoint.addresses()) {
          addressesForLocality.put(address, endpoint.endpointStatus());
        }
      }
      for (Map.Entry<String, EndpointStatus> addressEndpointStatus :
          addressesForLocality.entrySet()) {
        // LbEndpoints is mandatory.
        String address = addressEndpointStatus.getKey();
        var endpointStatus = addressEndpointStatus.getValue();
        localityLbEndpointsBuilder.addLbEndpoints(
            LbEndpoint.newBuilder()
                // Endpoint is mandatory.
                .setEndpoint(
                    Endpoint.newBuilder()
                        // Address is mandatory, must be unique within the cluster.
                        .setAddress(
                            Address.newBuilder()
                                .setSocketAddress(
                                    SocketAddress.newBuilder()
                                        .setAddress(address) // mandatory, IPv4 or IPv6
                                        .setPortValue(this.servingPort) // mandatory
                                        .setProtocolValue(SocketAddress.Protocol.TCP_VALUE)
                                        .build())
                                .build())
                        .build())
                .setHealthStatus(endpointStatus.getHealthStatus())
                .build());
      }
      clusterLoadAssignmentBuilder.addEndpoints(localityLbEndpointsBuilder.build());
    }
    // gRPC doesn't use the overprovisioning factor (effectively treats it as 100%), while the Envoy
    // default is 140%. See
    // https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/priority
    // and
    // https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/locality_weight
    clusterLoadAssignmentBuilder.setPolicy(
        Policy.newBuilder().setOverprovisioningFactor(UInt32Value.of(100)).build());
    return clusterLoadAssignmentBuilder.build();
  }
}
