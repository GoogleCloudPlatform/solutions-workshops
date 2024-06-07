// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rds

import (
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/lds"
)

// CreateRouteConfigurationForEnvoyGRPCListener returns an RDS route configuration for an Envoy
// proxy Listener that listens for gRPC requests.
func CreateRouteConfigurationForEnvoyGRPCListener(clusterNames []string) (*routev3.RouteConfiguration, error) {
	var virtualHosts []*routev3.VirtualHost
	for _, clusterName := range clusterNames {
		if strings.HasPrefix(clusterName, "xdstp://") {
			continue // skip clusters added for xDS federation
		}
		virtualHosts = append(virtualHosts, &routev3.VirtualHost{
			Name:    clusterName,
			Domains: []string{clusterName, clusterName + ".example.com", clusterName + ".xds.example.com"},
			Routes: []*routev3.Route{
				{
					Match: &routev3.RouteMatch{
						PathSpecifier: &routev3.RouteMatch_Prefix{
							Prefix: "",
						},
					},
					Action: &routev3.Route_Route{
						Route: &routev3.RouteAction{
							ClusterSpecifier: &routev3.RouteAction_Cluster{
								Cluster: clusterName,
							},
						},
					},
				},
			},
		})
	}
	routeConfiguration := routev3.RouteConfiguration{
		Name:         lds.EnvoyGRPCListenerRouteConfigurationName,
		VirtualHosts: virtualHosts,
	}
	return &routeConfiguration, nil
}
