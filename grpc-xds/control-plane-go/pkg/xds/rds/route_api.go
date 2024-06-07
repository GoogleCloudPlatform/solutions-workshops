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
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

// CreateRouteConfigurationForAPIListener returns an RDS route configuration for a gRPC
// client with one virtual host and one route for that virtual host.
//
// The virtual host Name is not used for routing.
// The request `:authority` must match one of the virtual host Domains.
// Te routePrefix parameter can be an empty string.
func CreateRouteConfigurationForAPIListener(name string, virtualHostName string, routePrefix string, clusterName string) *routev3.RouteConfiguration {
	return &routev3.RouteConfiguration{
		Name: name,
		VirtualHosts: []*routev3.VirtualHost{
			{
				Name:    virtualHostName,
				Domains: []string{"*"},
				Routes: []*routev3.Route{
					{
						Match: &routev3.RouteMatch{
							PathSpecifier: &routev3.RouteMatch_Prefix{
								Prefix: routePrefix,
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
			},
		},
	}
}
