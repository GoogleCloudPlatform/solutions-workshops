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
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	rbacv3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	faultv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/fault/v3"
	rbacfilterv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	envoyFilterHTTPFaultName       = "envoy.filters.http.fault"
	envoyFilterHTTPRBACName        = "envoy.filters.http.rbac"
	envoyFilterHTTPRouterName      = "envoy.filters.http.router"
	envoyHTTPConnectionManagerName = "envoy.http_connection_manager"
	envoyTransportSocketsTLSName   = "envoy.transport_sockets.tls"
	// serverListenerResourceNameTemplate uses the address and port above. Must match the template in the gRPC xDS bootstrap file, see
	// [gRFC A36: xDS-Enabled Servers]: https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol
	// Using the sample name from the gRPC-Go unit tests, but this is not important.
	serverListenerResourceNameTemplate = "grpc/server?xds.resource.listening_address=%s"
	// serverListenerRouteConfigurationName is used for the RouteConfiguration pointed to by server Listeners.
	serverListenerRouteConfigurationName = "default_inbound_config"
	// tlsCertificateProviderInstanceName is used in the `[Down|Up]streamTlsContext`s.
	// Using the same name as the `traffic-director-grpc-bootstrap` tool, but this is not important.
	// https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/2a9cf4614b56ec085c391a12f4cc53defaa575ac/main.go#L276
	tlsCertificateProviderInstanceName = "google_cloud_private_spiffe"
)

// SnapshotBuilder builds xDS resource snapshots for the cache.
type SnapshotBuilder struct {
	listeners               map[string]types.Resource
	routeConfigurations     map[string]types.Resource
	clusters                map[string]types.Resource
	clusterLoadAssignments  map[string]types.Resource
	endpointsByCluster      map[string][]GRPCApplicationEndpoints
	serverListenerAddresses map[EndpointAddress]bool
	nodeHash                string
	localityPriorityMapper  LocalityPriorityMapper
	features                *Features
	authority               string
}

// NewSnapshotBuilder initializes the builder.
func NewSnapshotBuilder(nodeHash string, localityPriorityMapper LocalityPriorityMapper, features *Features, authority string) *SnapshotBuilder {
	return &SnapshotBuilder{
		listeners:               make(map[string]types.Resource),
		routeConfigurations:     make(map[string]types.Resource),
		clusters:                make(map[string]types.Resource),
		clusterLoadAssignments:  make(map[string]types.Resource),
		endpointsByCluster:      make(map[string][]GRPCApplicationEndpoints),
		serverListenerAddresses: make(map[EndpointAddress]bool),
		nodeHash:                nodeHash,
		localityPriorityMapper:  localityPriorityMapper,
		features:                features,
		authority:               authority,
	}
}

// AddGRPCApplications adds the provided application configurations to the xDS resource snapshot.
func (b *SnapshotBuilder) AddGRPCApplications(apps []GRPCApplication) (*SnapshotBuilder, error) {
	for _, app := range apps {
		if b.listeners[app.ListenerName] == nil {
			apiListener, err := createAPIListener(app.ListenerName, app.ListenerName, app.RouteConfigurationName)
			if err != nil {
				return nil, fmt.Errorf("could not create LDS API listener for gRPC application %+v: %w", app, err)
			}
			b.listeners[apiListener.Name] = apiListener
			if b.features.EnableFederation {
				xdstpListenerName := xdstpListener(b.authority, app.ListenerName)
				xdstpRouteConfigurationName := xdstpRouteConfiguration(b.authority, app.RouteConfigurationName)
				xdstpListener, err := createAPIListener(xdstpListenerName, app.ListenerName, xdstpRouteConfigurationName)
				if err != nil {
					return nil, fmt.Errorf("could not create federation LDS API listener for authority=%s and gRPC application %+v: %w", b.authority, app, err)
				}
				b.listeners[xdstpListener.Name] = xdstpListener
			}
		}
		if b.routeConfigurations[app.RouteConfigurationName] == nil {
			routeConfiguration := createRouteConfiguration(app.RouteConfigurationName, app.ListenerName, app.PathPrefix, app.ClusterName)
			b.routeConfigurations[routeConfiguration.Name] = routeConfiguration
			if b.features.EnableFederation {
				xdstpRouteConfigurationName := xdstpRouteConfiguration(b.authority, app.RouteConfigurationName)
				xdstpClusterName := xdstpCluster(b.authority, app.ClusterName)
				xdstpRouteConfiguration := createRouteConfiguration(xdstpRouteConfigurationName, app.ListenerName, app.PathPrefix, xdstpClusterName)
				b.routeConfigurations[xdstpRouteConfiguration.Name] = xdstpRouteConfiguration
			}
		}
		if b.clusters[app.ClusterName] == nil {
			cluster, err := createCluster(
				app.ClusterName,
				app.EDSServiceName,
				app.Namespace,
				app.ServiceAccountName,
				b.features.EnableDataPlaneTLS,
				b.features.RequireDataPlaneClientCerts)
			if err != nil {
				return nil, fmt.Errorf("could not create CDS Cluster for gRPC application %+v: %w", app, err)
			}
			b.clusters[cluster.Name] = cluster
			if b.features.EnableFederation {
				xdstpClusterName := xdstpCluster(b.authority, app.ClusterName)
				xdstpEDSServiceName := xdstpEdsService(b.authority, app.EDSServiceName)
				xdstpCluster, err := createCluster(
					xdstpClusterName,
					xdstpEDSServiceName,
					app.Namespace,
					app.ServiceAccountName,
					b.features.EnableDataPlaneTLS,
					b.features.RequireDataPlaneClientCerts)
				if err != nil {
					return nil, fmt.Errorf("could not create federation CDS Cluster for authority=%s and gRPC application %+v: %w", b.authority, app, err)
				}
				b.clusters[xdstpCluster.Name] = xdstpCluster
			}
		}
		// Merge endpoints from multiple informers for the same app:
		endpointsByClusterKey := fmt.Sprintf("%s-%d", app.ClusterName, app.Port)
		b.endpointsByCluster[endpointsByClusterKey] = append(b.endpointsByCluster[endpointsByClusterKey], app.Endpoints...)
		clusterLoadAssignment := createClusterLoadAssignment(app.EDSServiceName, app.Port, b.nodeHash, b.localityPriorityMapper, b.endpointsByCluster[endpointsByClusterKey])
		b.clusterLoadAssignments[clusterLoadAssignment.ClusterName] = clusterLoadAssignment
		if b.features.EnableFederation {
			xdstpEDSServiceName := xdstpEdsService(b.authority, app.EDSServiceName)
			xdstpClusterLoadAssignment := createClusterLoadAssignment(xdstpEDSServiceName, app.Port, b.nodeHash, b.localityPriorityMapper, b.endpointsByCluster[endpointsByClusterKey])
			b.clusterLoadAssignments[xdstpClusterLoadAssignment.ClusterName] = xdstpClusterLoadAssignment
		}
	}
	return b, nil
}

func xdstpListener(authority string, listenerName string) string {
	return fmt.Sprintf("xdstp://%s/envoy.config.listener.v3.Listener/%s", authority, listenerName)
}

func xdstpRouteConfiguration(authority string, routeConfigurationName string) string {
	return fmt.Sprintf("xdstp://%s/envoy.config.route.v3.RouteConfiguration/%s", authority, routeConfigurationName)
}

func xdstpCluster(authority string, clusterName string) string {
	return fmt.Sprintf("xdstp://%s/envoy.config.cluster.v3.Cluster/%s", authority, clusterName)
}

func xdstpEdsService(authority string, serviceName string) string {
	return fmt.Sprintf("xdstp://%s/envoy.config.endpoint.v3.ClusterLoadAssignment/%s", authority, serviceName)
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
	rbacPerRouteConfig, err := createRBACPerRouteConfig("xds", "host-certs")
	if err != nil {
		return nil, fmt.Errorf("could not marshall RBACPerRoute typedConfig into Any instance: %w", err)
	}
	for address := range b.serverListenerAddresses {
		serverListener, err := createServerListener(
			address.Host,
			address.Port,
			serverListenerRouteConfigurationName,
			rbacPerRouteConfig,
			b.features.ServerListenerUsesRDS,
			b.features.EnableDataPlaneTLS,
			b.features.RequireDataPlaneClientCerts)
		if err != nil {
			return nil, fmt.Errorf("could not create server Listener for address %s:%d: %w", address.Host, address.Port, err)
		}
		b.listeners[serverListener.Name] = serverListener
	}
	if len(b.serverListenerAddresses) > 0 {
		routeConfigurationForServerListener := createRouteConfigurationForServerListener(serverListenerRouteConfigurationName, rbacPerRouteConfig)
		b.routeConfigurations[routeConfigurationForServerListener.Name] = routeConfigurationForServerListener
	}

	listeners := make([]types.Resource, len(b.listeners))
	i := 0
	for _, listener := range b.listeners {
		listeners[i] = listener
		i++
	}
	routeConfigurations := make([]types.Resource, len(b.routeConfigurations))
	j := 0
	for _, routeConfiguration := range b.routeConfigurations {
		routeConfigurations[j] = routeConfiguration
		j++
	}
	clusters := make([]types.Resource, len(b.clusters))
	k := 0
	for _, cluster := range b.clusters {
		clusters[k] = cluster
		k++
	}
	clusterLoadAssignments := make([]types.Resource, len(b.clusterLoadAssignments))
	l := 0
	for _, clusterLoadAssignment := range b.clusterLoadAssignments {
		clusterLoadAssignments[l] = clusterLoadAssignment
		l++
	}

	version := strconv.FormatInt(time.Now().UnixNano(), 10)
	return cachev3.NewSnapshot(version, map[resource.Type][]types.Resource{
		resource.ListenerType: listeners,
		resource.RouteType:    routeConfigurations,
		resource.ClusterType:  clusters,
		resource.EndpointType: clusterLoadAssignments,
	})
}

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
								Rule: &rbacv3.Permission_Any{
									Any: true,
								},
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

// createAPIListener returns an LDS API listener
//
// [gRFC A27]: https://github.com/grpc/proposal/blob/master/A27-xds-global-load-balancing.md#listener-proto
// [Reference]: https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/listener/v3/api_listener.proto
func createAPIListener(name string, statPrefix string, routeConfigurationName string) (*listenerv3.Listener, error) {
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
		StatPrefix: statPrefix,
		RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
			Rds: &hcmv3.Rds{
				ConfigSource: &corev3.ConfigSource{
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
						Ads: &corev3.AggregatedConfigSource{},
					},
					ResourceApiVersion: corev3.ApiVersion_V3,
				},
				RouteConfigName: routeConfigurationName,
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
func createServerListener(host string, port uint32, routeConfigurationName string, rbacPerRouteConfig *anypb.Any, useRDS bool, enableTLS bool, requireClientCerts bool) (*listenerv3.Listener, error) {
	routerTypedConfig, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("could not marshall Router HTTP filter typedConfig into Any instance: %w", err)
	}
	rbacFilterTypedConfig, err := anypb.New(&rbacfilterv3.RBAC{
		Rules: &rbacv3.RBAC{}, // Present and empty `Rules` mean DENY all. Override per route.
	})
	if err != nil {
		return nil, fmt.Errorf("could not marshall RBAC HTTP filter typedConfig into Any instance: %w", err)
	}
	httpConnectionManager := createHTTPConnectionManagerForServerListener(routeConfigurationName, routerTypedConfig, rbacFilterTypedConfig, rbacPerRouteConfig, useRDS, enableTLS, requireClientCerts)
	anyWrappedHTTPConnectionManager, err := anypb.New(httpConnectionManager)
	if err != nil {
		return nil, fmt.Errorf("could not marshall HttpConnectionManager +%v into Any instance: %w", httpConnectionManager, err)
	}

	// Relying on `serverListenerResourceNameTemplate` being an exact match of
	// `server_listener_resource_name_template` from the gRPC xDS bootstrap configuration. See
	// [gRFC A36: xDS-Enabled Servers]: https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol
	listenerName := fmt.Sprintf(serverListenerResourceNameTemplate, net.JoinHostPort(host, strconv.Itoa(int(port))))

	serverListener := listenerv3.Listener{
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
						Name: envoyHTTPConnectionManagerName, // must be the last filter
						ConfigType: &listenerv3.Filter_TypedConfig{
							TypedConfig: anyWrappedHTTPConnectionManager,
						},
					},
				},
			},
		},
		TrafficDirection: corev3.TrafficDirection_INBOUND,
		EnableReusePort:  wrapperspb.Bool(true),
	}

	if enableTLS {
		downstreamTLSContext := createDownstreamTLSContext(requireClientCerts)
		anyWrappedDownstreamTLSContext, err := anypb.New(downstreamTLSContext)
		if err != nil {
			return nil, fmt.Errorf("could not marshall DownstreamTlsContext %+v into Any instance: %w", downstreamTLSContext, err)
		}
		serverListener.FilterChains[0].TransportSocket = &corev3.TransportSocket{
			Name: envoyTransportSocketsTLSName,
			ConfigType: &corev3.TransportSocket_TypedConfig{
				TypedConfig: anyWrappedDownstreamTLSContext,
			},
		}
	}

	return &serverListener, nil
}

func createHTTPConnectionManagerForServerListener(routeConfigurationName string, routerFilterTypedConfig *anypb.Any, rbacFilterTypedConfig *anypb.Any, rbacPerRouteTypedConfig *anypb.Any, useRDS bool, enableTLS bool, requireClientCerts bool) *hcmv3.HttpConnectionManager {
	httpConnectionManager := hcmv3.HttpConnectionManager{
		CodecType:  hcmv3.HttpConnectionManager_AUTO,
		StatPrefix: routeConfigurationName,
		HttpFilters: []*hcmv3.HttpFilter{
			{
				// Router must be the last filter.
				Name: envoyFilterHTTPRouterName,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{
					TypedConfig: routerFilterTypedConfig,
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

	if useRDS {
		// Dynamic RouteConfiguration via RDS for server listeners:
		httpConnectionManager.RouteSpecifier = &hcmv3.HttpConnectionManager_Rds{
			Rds: &hcmv3.Rds{
				ConfigSource: &corev3.ConfigSource{
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
						Ads: &corev3.AggregatedConfigSource{},
					},
					ResourceApiVersion: corev3.ApiVersion_V3,
				},
				RouteConfigName: routeConfigurationName,
			},
		}
	} else {
		// Inline RouteConfiguration for server listeners:
		httpConnectionManager.RouteSpecifier = &hcmv3.HttpConnectionManager_RouteConfig{
			RouteConfig: createRouteConfigurationForServerListener(routeConfigurationName, rbacPerRouteTypedConfig),
		}
	}

	if enableTLS && requireClientCerts {
		// Prepend RBAC HTTP filter. Not append, as Router must be the last HTTP filter.
		httpConnectionManager.HttpFilters = append([]*hcmv3.HttpFilter{
			{
				Name: envoyFilterHTTPRBACName,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{
					TypedConfig: rbacFilterTypedConfig,
				},
			},
		}, httpConnectionManager.HttpFilters...)
	}

	return &httpConnectionManager
}

// createDownstreamTLSContext configures:
// 1. gRPC server TLS certificate provider
// 2. certificate authorities (CAs) to validate gRPC client certificates.
func createDownstreamTLSContext(requireClientCerts bool) *tlsv3.DownstreamTlsContext {
	downstreamTLSContext := tlsv3.DownstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			// Set server certificate:
			TlsCertificateProviderInstance: &tlsv3.CertificateProviderPluginInstance{
				InstanceName: tlsCertificateProviderInstanceName,
				// Using the same certificate name value as Traffic Director, but the
				// certificate name is ignored by gRPC according to gRFC A29.
				CertificateName: "DEFAULT",
			},
			// AlpnProtocols is set by Traffic Director, but ignored by gRPC xDS according to gRFC A29.
			AlpnProtocols: []string{"h2"},
		},
	}

	if requireClientCerts {
		// `require_client_certificate: true` requires a `validation_context`.
		downstreamTLSContext.RequireClientCertificate = wrapperspb.Bool(true)
		// Validate client certificates:
		// gRFC A29 specifies to use either `validation_context` or
		// `combined_validation_context.default_validation_context`, but
		// gRPC-Java as of v1.60.0 doesn't handle `combined_validation_context` correctly.
		downstreamTLSContext.CommonTlsContext.ValidationContextType = &tlsv3.CommonTlsContext_ValidationContext{
			ValidationContext: &tlsv3.CertificateValidationContext{
				CaCertificateProviderInstance: &tlsv3.CertificateProviderPluginInstance{
					InstanceName: tlsCertificateProviderInstanceName,
					// Using the same certificate name value as Traffic Director,
					// but the certificate name is ignored by gRPC, see gRFC A29.
					CertificateName: "ROOTCA",
				},
			},
		}
	}

	return &downstreamTLSContext
}

// createRouteConfiguration returns an RDS route configuration for a gRPC
// application with one virtual host and one route for that virtual host.
//
// The virtual host Name is not used for routing.
// The virtual host domain must match the request `:authority`
// Te routePrefix parameter can be an empty string.
func createRouteConfiguration(name string, virtualHostName string, routePrefix string, clusterName string) *routev3.RouteConfiguration {
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

// createRouteConfigurationForServerListener returns an RDS route configuration for the server listeners.
func createRouteConfigurationForServerListener(name string, rbacPerRouteConfig *anypb.Any) *routev3.RouteConfiguration {
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
						TypedPerFilterConfig: map[string]*anypb.Any{
							envoyFilterHTTPRBACName: rbacPerRouteConfig,
						},
					},
				},
			},
		},
	}
}

// createCluster definition for CDS.
// [gRFC A27]: https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#cluster-proto
func createCluster(name string, edsServiceName string, namespace string, serviceAccountName string, enableTLS bool, requireClientCerts bool) (*clusterv3.Cluster, error) {
	cluster := clusterv3.Cluster{
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
			ServiceName: edsServiceName,
		},
		ConnectTimeout: &durationpb.Duration{
			Seconds: 3, // default is 5s
		},
		LbPolicy: clusterv3.Cluster_ROUND_ROBIN,
	}

	if enableTLS {
		upstreamTLSContext := createUpstreamTLSContext(namespace, serviceAccountName, requireClientCerts)
		anyWrappedUpstreamTLSContext, err := anypb.New(upstreamTLSContext)
		if err != nil {
			return nil, fmt.Errorf("could not marshall UpstreamTlsContext +%v into Any instance: %w", upstreamTLSContext, err)
		}
		cluster.TransportSocket = &corev3.TransportSocket{
			Name: envoyTransportSocketsTLSName,
			ConfigType: &corev3.TransportSocket_TypedConfig{
				TypedConfig: anyWrappedUpstreamTLSContext,
			},
		}
	}

	return &cluster, nil
}

// createUpstreamTLSContext configures:
// 1. gRPC client TLS certificate provider
// 2. certificate authorities (CAs) to validate gRPC server certificates, including server authorization.
// Important: Assumes that the client application k8s Service account name matches the application name!
func createUpstreamTLSContext(namespace string, serviceAccountName string, requireClientCerts bool) *tlsv3.UpstreamTlsContext {
	upstreamTLSContext := tlsv3.UpstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			// Validate gRPC server certificates:
			ValidationContextType: &tlsv3.CommonTlsContext_ValidationContext{
				ValidationContext: &tlsv3.CertificateValidationContext{
					CaCertificateProviderInstance: &tlsv3.CertificateProviderPluginInstance{
						InstanceName: tlsCertificateProviderInstanceName,
						// Using the same certificate name value as Traffic Director,
						// but the certificate name is ignored by gRPC, see gRFC A29.
						CertificateName: "ROOTCA",
					},
					// Server authorization (SAN checks):
					// gRPC-Java as of v1.60.0 does not work correctly with
					// `match_typed_subject_alt_names`, using `match_subject_alt_names`
					// instead, for now.
					MatchSubjectAltNames: []*matcherv3.StringMatcher{
						{
							MatchPattern: &matcherv3.StringMatcher_SafeRegex{
								SafeRegex: &matcherv3.RegexMatcher{
									Regex: fmt.Sprintf("spiffe://[^/]+/ns/%s/sa/%s", namespace, serviceAccountName),
								},
							},
						},
					},
				},
			},
			// AlpnProtocols is set by Traffic Director, but ignored by gRPC xDS according to gRFC A29.
			AlpnProtocols: []string{"h2"},
		},
	}

	if requireClientCerts {
		// Send client certificate in TLS handshake:
		upstreamTLSContext.CommonTlsContext.TlsCertificateProviderInstance = &tlsv3.CertificateProviderPluginInstance{
			InstanceName: tlsCertificateProviderInstanceName,
			// Using the same certificate name value as Traffic Director, but the
			// certificate name is ignored by gRPC according to gRFC A29.
			CertificateName: "DEFAULT",
		}
	}

	return &upstreamTLSContext
}

// createClusterLoadAssignment for EDS.
// `edsServiceName` must match the `ServiceName` in the `EDSClusterConfig` in the CDS Cluster resource.
// [gRFC A27]: https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md#clusterloadassignment-proto
func createClusterLoadAssignment(edsServiceName string, port uint32, nodeHash string, localityPriorityMapper LocalityPriorityMapper, endpoints []GRPCApplicationEndpoints) *endpointv3.ClusterLoadAssignment {
	// addressesByZone := map[string][]string{}
	// for _, endpoint := range endpoints {
	//	addressesByZone[endpoint.Zone] = append(addressesByZone[endpoint.Zone], endpoint.Addresses...)
	//}
	endpointsByZone := map[string][]GRPCApplicationEndpoints{}
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
