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

package xds

import (
	"fmt"
	"net"
	"strconv"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	faultv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/fault/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	envoyHTTPConnectionManagerName = "envoy.http_connection_manager"
	envoyFilterHTTPFaultName       = "envoy.filters.http.fault"
	envoyFilterHTTPRouterName      = "envoy.filters.http.router"
	// serverListenerResourceNameTemplate uses the address and port above. Must match the template in the gRPC xDS bootstrap file, see
	// [gRFC A36: xDS-Enabled Servers]: https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol
	// Using the sample name from the gRPC-Go unit tests, but this is not important.
	serverListenerResourceNameTemplate = "grpc/server?xds.resource.listening_address=%s"
	// serverListenerRouteConfigName is used for the RouteConfiguration pointed to by server Listeners.
	serverListenerRouteConfigName = "default_inbound_config"
)

// SnapshotBuilder builds xDS resource snapshots for the cache.
type SnapshotBuilder struct {
	listeners               map[string]types.Resource
	routeConfigs            map[string]types.Resource
	clusters                map[string]types.Resource
	clusterLoadAssignments  map[string]types.Resource
	serverListenerAddresses map[EndpointAddress]bool
}

// NewSnapshotBuilder initializes the builder.
func NewSnapshotBuilder() *SnapshotBuilder {
	return &SnapshotBuilder{
		listeners:               make(map[string]types.Resource),
		routeConfigs:            make(map[string]types.Resource),
		clusters:                make(map[string]types.Resource),
		clusterLoadAssignments:  make(map[string]types.Resource),
		serverListenerAddresses: make(map[EndpointAddress]bool),
	}
}

// AddSnapshot adds Listener, RouteConfiguration, Cluster, and
// ClusterLoadAssignment resources from the provided snapshot to the builder.
func (b *SnapshotBuilder) AddSnapshot(snapshot cachev3.ResourceSnapshot) *SnapshotBuilder {
	if snapshot == nil {
		return b
	}
	for name, listener := range snapshot.GetResources(resource.ListenerType) {
		b.listeners[name] = listener
	}
	for name, routeConfig := range snapshot.GetResources(resource.RouteType) {
		b.routeConfigs[name] = routeConfig
	}
	for name, cluster := range snapshot.GetResources(resource.ClusterType) {
		b.clusters[name] = cluster
	}
	for name, clusterLoadAssignment := range snapshot.GetResources(resource.EndpointType) {
		b.clusterLoadAssignments[name] = clusterLoadAssignment
	}
	return b
}

// AddGRPCApplications adds the provided application configurations to the xDS
// resource snapshot.
//
// TODO: There can be more than one EndpointSlice for a k8s Service.
// Check if there's already an application with the same name and merge,
// instead of just blindly overwriting.
func (b *SnapshotBuilder) AddGRPCApplications(apps []GRPCApplication) (*SnapshotBuilder, error) {
	for _, app := range apps {
		apiListener, err := createAPIListener(app.listenerName(), app.routeConfigName())
		if err != nil {
			return nil, fmt.Errorf("could not create LDS API listener for gRPC application %+v: %w", app, err)
		}
		b.listeners[apiListener.Name] = apiListener
		routeConfig := createRouteConfig(app.routeConfigName(), app.listenerName(), app.PathPrefix, app.clusterName())
		b.routeConfigs[routeConfig.Name] = routeConfig
		cluster := createCluster(app.clusterName())
		b.clusters[cluster.Name] = cluster
		clusterLoadAssignment := createClusterLoadAssignment(app.clusterName(), app.Port, app.Endpoints)
		b.clusterLoadAssignments[clusterLoadAssignment.ClusterName] = clusterLoadAssignment
	}
	return b, nil
}

// AddServerListenerAddresses adds server listeners and associated route
// configurations with the provided IP addresses and ports to the snapshot.
func (b *SnapshotBuilder) AddServerListenerAddresses(addresses []EndpointAddress) *SnapshotBuilder {
	for _, address := range addresses {
		b.serverListenerAddresses[address] = true
	}
	return b
}

// Build adds the server listeners and route configuration for the node hash, and then builds the snapshot.
func (b *SnapshotBuilder) Build() (cachev3.ResourceSnapshot, error) {
	for address := range b.serverListenerAddresses {
		serverListener, err := createServerListener(address.host, address.port, serverListenerRouteConfigName)
		if err != nil {
			return nil, fmt.Errorf("could not create server Listener for address %s:%d: %w", address.host, address.port, err)
		}
		b.listeners[serverListener.Name] = serverListener
	}
	if len(b.serverListenerAddresses) > 0 {
		routeConfigForServerListener := createRouteConfigForServerListener(serverListenerRouteConfigName)
		b.routeConfigs[routeConfigForServerListener.Name] = routeConfigForServerListener
	}

	listeners := make([]types.Resource, len(b.listeners))
	i := 0
	for _, listener := range b.listeners {
		listeners[i] = listener
		i++
	}
	routeConfigs := make([]types.Resource, len(b.routeConfigs))
	i = 0
	for _, routeConfig := range b.routeConfigs {
		routeConfigs[i] = routeConfig
		i++
	}
	clusters := make([]types.Resource, len(b.clusters))
	i = 0
	for _, cluster := range b.clusters {
		clusters[i] = cluster
		i++
	}
	clusterLoadAssignments := make([]types.Resource, len(b.clusterLoadAssignments))
	i = 0
	for _, clusterLoadAssignment := range b.clusterLoadAssignments {
		clusterLoadAssignments[i] = clusterLoadAssignment
		i++
	}

	version := strconv.FormatInt(time.Now().UnixNano(), 10)
	return cachev3.NewSnapshot(version, map[resource.Type][]types.Resource{
		resource.ListenerType: listeners,
		resource.RouteType:    routeConfigs,
		resource.ClusterType:  clusters,
		resource.EndpointType: clusterLoadAssignments,
	})
}

// createAPIListener returns an LDS API listener
//
// [gRFC A27]: https://github.com/grpc/proposal/blob/master/A27-xds-global-load-balancing.md#listener-proto
// [Reference]: https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/listener/v3/api_listener.proto
func createAPIListener(name string, routeConfigName string) (*listenerv3.Listener, error) {
	httpFaultFilterTypedConfig, err := anypb.New(&faultv3.HTTPFault{})
	if err != nil {
		return nil, fmt.Errorf("could not marshall HTTPFault typedConfig into Any instance: %w", err)
	}
	routerFilterTypedConfig, err := anypb.New(&routerv3.Router{
		SuppressEnvoyHeaders: true,
	})
	if err != nil {
		return nil, fmt.Errorf("could not marshall Router typedConfig into Any instance: %w", err)
	}
	httpConnectionManager := &hcmv3.HttpConnectionManager{
		CodecType: hcmv3.HttpConnectionManager_AUTO,
		// https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/stats#config-http-conn-man-stats
		StatPrefix: name,
		RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
			Rds: &hcmv3.Rds{
				ConfigSource: &corev3.ConfigSource{
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
						Ads: &corev3.AggregatedConfigSource{},
					},
					ResourceApiVersion: corev3.ApiVersion_V3,
				},
				RouteConfigName: routeConfigName,
			},
		},
		HttpFilters: []*hcmv3.HttpFilter{
			{
				// Enable fault injection.
				Name: envoyFilterHTTPFaultName,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{
					TypedConfig: httpFaultFilterTypedConfig,
				},
			},
			{
				// Router must be the last filter.
				Name: envoyFilterHTTPRouterName,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{
					TypedConfig: routerFilterTypedConfig,
				},
			},
		},
	}
	anyWrappedHTTPConnectionManager, err := anypb.New(httpConnectionManager)
	if err != nil {
		return nil, fmt.Errorf("could not marshall HttpConnectionManager +%v into Any instance: %w", httpConnectionManager, err)
	}
	return &listenerv3.Listener{
		Name: name,
		ApiListener: &listenerv3.ApiListener{
			ApiListener: anyWrappedHTTPConnectionManager,
		},
	}, nil
}

// createServerListener returns a listener for xDS clients that serve gRPC services.
func createServerListener(host string, port uint32, routeConfigName string) (*listenerv3.Listener, error) {
	routerTypedConfig, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("could not marshall Router typedConfig into Any instance: %w", err)
	}
	httpConnectionManager := createHTTPConnectionManagerForServerListener(routeConfigName, routerTypedConfig)
	anyWrappedHTTPConnectionManager, err := anypb.New(httpConnectionManager)
	if err != nil {
		return nil, fmt.Errorf("could not marshall HttpConnectionManager +%v into Any instance: %w", httpConnectionManager, err)
	}

	// Relying on `serverListenerResourceNameTemplate` being an exact match of
	// `server_listener_resource_name_template` from the gRPC xDS bootstrap configuration. See
	// [gRFC A36: xDS-Enabled Servers]: https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol
	listenerName := fmt.Sprintf(serverListenerResourceNameTemplate, net.JoinHostPort(host, strconv.Itoa(int(port))))

	return &listenerv3.Listener{
		Name: listenerName,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address: host,
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: port,
					},
					Protocol: corev3.SocketAddress_TCP,
				},
			},
		},
		FilterChains: []*listenerv3.FilterChain{
			{
				Filters: []*listenerv3.Filter{
					{
						Name: envoyHTTPConnectionManagerName,
						ConfigType: &listenerv3.Filter_TypedConfig{
							TypedConfig: anyWrappedHTTPConnectionManager,
						},
					},
				},
			},
		},
		TrafficDirection: corev3.TrafficDirection_INBOUND,
		EnableReusePort:  wrapperspb.Bool(true),
	}, nil
}

func createHTTPConnectionManagerForServerListener(routeConfigName string, routerTypedConfig *anypb.Any) *hcmv3.HttpConnectionManager {
	return &hcmv3.HttpConnectionManager{
		CodecType:  hcmv3.HttpConnectionManager_AUTO,
		StatPrefix: routeConfigName,
		// Inline RouteConfiguration for server listeners:
		RouteSpecifier: &hcmv3.HttpConnectionManager_RouteConfig{
			RouteConfig: createRouteConfigForServerListener(routeConfigName),
		},
		// Dynamic RouteConfiguration via RDS for server listeners:
		// RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
		// 	Rds: &hcmv3.Rds{
		// 		ConfigSource: &corev3.ConfigSource{
		// 			ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
		// 				Ads: &corev3.AggregatedConfigSource{},
		// 			},
		// 			ResourceApiVersion: corev3.ApiVersion_V3,
		// 		},
		// 		RouteConfigName: routeConfigName,
		// 	},
		// },
		HttpFilters: []*hcmv3.HttpFilter{
			{
				// Router must be the last filter.
				Name: envoyFilterHTTPRouterName,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{
					TypedConfig: routerTypedConfig,
				},
			},
		},
		ForwardClientCertDetails: hcmv3.HttpConnectionManager_APPEND_FORWARD,
		SetCurrentClientCertDetails: &hcmv3.HttpConnectionManager_SetCurrentClientCertDetails{
			Subject: wrapperspb.Bool(true),
			Dns:     true,
			Uri:     true,
		},
		UpgradeConfigs: []*hcmv3.HttpConnectionManager_UpgradeConfig{
			{
				UpgradeType: "websocket",
			},
		},
	}
}

// createRouteConfig returns an RDS route configuration for a gRPC
// application with one virtual host and one route for that virtual host.
//
// The virtual host Name is not used for routing.
// The virtual host domain must match the request `:authority`
// Te routePrefix parameter can be an empty string.
func createRouteConfig(name string, virtualHostName string, routePrefix string, clusterName string) *routev3.RouteConfiguration {
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

// createRouteConfigForServerListener returns an RDS route configuration for the server listeners.
func createRouteConfigForServerListener(name string) *routev3.RouteConfiguration {
	return &routev3.RouteConfiguration{
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
}

// createCluster definition for CDS.
// [gRFC A27]: https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#cluster-proto
func createCluster(name string) *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name: name,
		ClusterDiscoveryType: &clusterv3.Cluster_Type{
			Type: clusterv3.Cluster_EDS,
		},
		EdsClusterConfig: &clusterv3.Cluster_EdsClusterConfig{
			EdsConfig: &corev3.ConfigSource{
				ResourceApiVersion: corev3.ApiVersion_V3,
				ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
					Ads: &corev3.AggregatedConfigSource{},
				},
			},
		},
		ConnectTimeout: &durationpb.Duration{
			Seconds: 3, // default is 5s
		},
		LbPolicy: clusterv3.Cluster_ROUND_ROBIN,
	}
}

// createClusterLoadAssignment for EDS.
// [gRFC A27]: https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#clusterloadassignment-proto
func createClusterLoadAssignment(clusterName string, port uint32, endpoints []GRPCApplicationEndpoints) *endpointv3.ClusterLoadAssignment {
	addressesByZone := map[string][]string{}
	for _, endpoint := range endpoints {
		addressesByZone[endpoint.Zone] = append(addressesByZone[endpoint.Zone], endpoint.Addresses...)
	}
	cla := &endpointv3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   []*endpointv3.LocalityLbEndpoints{},
	}
	for zone, addresses := range addressesByZone {
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
			Priority: 0,
		}
		for _, address := range addresses {
			localityLbEndpoints.LbEndpoints = append(localityLbEndpoints.LbEndpoints,
				&endpointv3.LbEndpoint{
					// HealthStatus should be HEALTHY or UNKNOWN
					HealthStatus: corev3.HealthStatus_HEALTHY,
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
		cla.Endpoints = append(cla.Endpoints, localityLbEndpoints)
	}
	return cla
}
