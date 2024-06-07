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

package eds

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/applications"
)

// CreateClusterLoadAssignment for EDS.
// `edsServiceName` must match the `ServiceName` in the `EDSClusterConfig` in the CDS Cluster resource.
// [gRFC A27]: https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#clusterloadassignment-proto
func CreateClusterLoadAssignment(edsServiceName string, port uint32, nodeHash string, localityPriorityMapper LocalityPriorityMapper, endpoints []applications.ApplicationEndpoints) *endpointv3.ClusterLoadAssignment {
	// addressesByZone := map[string][]string{}
	// for _, endpoint := range endpoints {
	//	addressesByZone[endpoint.Zone] = append(addressesByZone[endpoint.Zone], endpoint.Addresses...)
	// }
	endpointsByZone := map[string][]applications.ApplicationEndpoints{}
	for _, endpoint := range endpoints {
		endpointsByZone[endpoint.Zone] = append(endpointsByZone[endpoint.Zone], endpoint)
	}
	zones := make([]string, len(endpointsByZone))
	i := 0
	for zone := range endpointsByZone {
		zones[i] = zone
		i++
	}
	zonePriorities := localityPriorityMapper.BuildPriorityMap(nodeHash, zones)
	cla := &endpointv3.ClusterLoadAssignment{
		ClusterName: edsServiceName,
		Endpoints:   []*endpointv3.LocalityLbEndpoints{},
	}
	for zone, endpoints := range endpointsByZone {
		localityLbEndpoints := &endpointv3.LocalityLbEndpoints{
			// LbEndpoints is mandatory.
			LbEndpoints: []*endpointv3.LbEndpoint{},
			// Weight is effectively mandatory, read the javadoc carefully :-)
			LoadBalancingWeight: wrapperspb.UInt32(100000),
			// Locality must be unique for a given priority.
			Locality: &corev3.Locality{
				Zone: zone,
			},
			// Priority is optional. If provided, must start from 0 and have no gaps.
			Priority: zonePriorities[zone],
		}
		for _, endpoint := range endpoints {
			for _, address := range endpoint.Addresses {
				localityLbEndpoints.LbEndpoints = append(localityLbEndpoints.LbEndpoints,
					&endpointv3.LbEndpoint{
						HealthStatus: endpoint.EndpointStatus.HealthStatus(),
						HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
							// Endpoint is mandatory.
							Endpoint: &endpointv3.Endpoint{
								// Address is mandatory, must be unique within the cluster.
								Address: &corev3.Address{
									Address: &corev3.Address_SocketAddress{
										SocketAddress: &corev3.SocketAddress{
											Protocol: corev3.SocketAddress_TCP,
											Address:  address, // mandatory, IPv4 or IPv6
											PortSpecifier: &corev3.SocketAddress_PortValue{
												PortValue: port, // mandatory
											},
										},
									},
								},
							},
						},
					})
			}
		}
		cla.Endpoints = append(cla.Endpoints, localityLbEndpoints)
	}
	return cla
}
