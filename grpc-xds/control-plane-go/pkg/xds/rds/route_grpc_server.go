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
	"fmt"
	"strings"

	rbacv3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	rbacfilterv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/lds"
)

// CreateRouteConfigurationForGRPCServerListener returns an RDS route configuration for a gRPC server Listener.
func CreateRouteConfigurationForGRPCServerListener(enableRBAC bool) (*routev3.RouteConfiguration, error) {
	name := lds.GRPCServerListenerRouteConfigurationName
	routeConfiguration := routev3.RouteConfiguration{
		Name: name,
		VirtualHosts: []*routev3.VirtualHost{
			{
				// The VirtualHost name _doesn't_ have to match the RouteConfiguration name.
				Name:    name,
				Domains: []string{"*"},
				Routes: []*routev3.Route{
					{
						Match: &routev3.RouteMatch{
							PathSpecifier: &routev3.RouteMatch_Prefix{
								Prefix: "/",
							},
						},
						Action: &routev3.Route_NonForwardingAction{
							NonForwardingAction: &routev3.NonForwardingAction{},
						},
						Decorator: &routev3.Decorator{
							Operation: name + "/*",
						},
					},
				},
			},
		},
	}
	if enableRBAC {
		rbacPerRouteConfig, err := createRBACPerRouteConfig("xds", "host-certs")
		if err != nil {
			return nil, fmt.Errorf("could not marshall RBACPerRoute typedConfig into Any instance: %w", err)
		}
		for _, virtualHost := range routeConfiguration.VirtualHosts {
			for _, route := range virtualHost.Routes {
				route.TypedPerFilterConfig = map[string]*anypb.Any{
					lds.EnvoyFilterHTTPRBACName: rbacPerRouteConfig,
				}
			}
		}
	}
	return &routeConfiguration, nil
}

// createRBACPerRouteConfig returns an RBACPerRoute config with a single policy called
// `greeter-clients`. The policy applies to all URL paths, and it permits workloads with a
// X.509 SVID for any Kubernetes ServiceAccount in all the specified Kubernetes Namespaces.
func createRBACPerRouteConfig(allowNamespaces ...string) (*anypb.Any, error) {
	pipedNamespaces := strings.Join(allowNamespaces, "|")
	return anypb.New(&rbacfilterv3.RBACPerRoute{
		Rbac: &rbacfilterv3.RBAC{
			Rules: &rbacv3.RBAC{
				Action: rbacv3.RBAC_ALLOW,
				Policies: map[string]*rbacv3.Policy{
					"greeter-clients": {
						Permissions: []*rbacv3.Permission{
							{
								// Permissions can match URL path, headers/metadata, and more.
								Rule: &rbacv3.Permission_UrlPath{
									UrlPath: &matcherv3.PathMatcher{
										Rule: &matcherv3.PathMatcher_Path{
											Path: &matcherv3.StringMatcher{
												MatchPattern: &matcherv3.StringMatcher_Prefix{
													Prefix: "/helloworld.Greeter/",
												},
												IgnoreCase: true,
											},
										},
									},
								},
								// Rule: &rbacv3.Permission_Any{
								// 	Any: true,
								// },
							},
						},
						Principals: []*rbacv3.Principal{
							{
								Identifier: &rbacv3.Principal_Authenticated_{
									Authenticated: &rbacv3.Principal_Authenticated{
										PrincipalName: &matcherv3.StringMatcher{
											MatchPattern: &matcherv3.StringMatcher_SafeRegex{
												SafeRegex: &matcherv3.RegexMatcher{
													// Matches against URI SANs, then DNS SANs, then Subject DN.
													Regex: fmt.Sprintf("spiffe://[^/]+/ns/(%s)/sa/.+", pipedNamespaces),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
}
