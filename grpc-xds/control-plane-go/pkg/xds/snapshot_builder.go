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
	"strconv"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/applications"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/cds"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/eds"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/lds"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/rds"
)

// SnapshotBuilder builds xDS resource snapshots for the cache.
type SnapshotBuilder struct {
	listeners                   map[string]types.Resource
	routeConfigurations         map[string]types.Resource
	clusters                    map[string]types.Resource
	clusterLoadAssignments      map[string]types.Resource
	endpointsByCluster          map[string][]applications.ApplicationEndpoints
	grpcServerListenerAddresses map[EndpointAddress]bool
	nodeHash                    string
	localityPriorityMapper      eds.LocalityPriorityMapper
	features                    *Features
	authority                   string
}

// NewSnapshotBuilder initializes the builder.
func NewSnapshotBuilder(nodeHash string, localityPriorityMapper eds.LocalityPriorityMapper, features *Features, authority string) *SnapshotBuilder {
	return &SnapshotBuilder{
		listeners:                   make(map[string]types.Resource),
		routeConfigurations:         make(map[string]types.Resource),
		clusters:                    make(map[string]types.Resource),
		clusterLoadAssignments:      make(map[string]types.Resource),
		endpointsByCluster:          make(map[string][]applications.ApplicationEndpoints),
		grpcServerListenerAddresses: make(map[EndpointAddress]bool),
		nodeHash:                    nodeHash,
		localityPriorityMapper:      localityPriorityMapper,
		features:                    features,
		authority:                   authority,
	}
}

// AddGRPCApplications adds the provided application configurations to the xDS resource snapshot.
func (b *SnapshotBuilder) AddGRPCApplications(apps []applications.Application) (*SnapshotBuilder, error) {
	for _, app := range apps {
		if b.listeners[app.ListenerName] == nil {
			apiListener, err := lds.CreateAPIListener(app.ListenerName, app.RouteConfigurationName)
			if err != nil {
				return nil, fmt.Errorf("could not create LDS API listener for gRPC application %+v: %w", app, err)
			}
			b.listeners[apiListener.Name] = apiListener
			if b.features.EnableFederation {
				xdstpListenerName := xdstpListener(b.authority, app.ListenerName)
				xdstpRouteConfigurationName := xdstpRouteConfiguration(b.authority, app.RouteConfigurationName)
				xdstpListener, err := lds.CreateAPIListener(xdstpListenerName, xdstpRouteConfigurationName)
				if err != nil {
					return nil, fmt.Errorf("could not create federation LDS API listener for authority=%s and gRPC application %+v: %w", b.authority, app, err)
				}
				b.listeners[xdstpListener.Name] = xdstpListener
			}
		}
		if b.routeConfigurations[app.RouteConfigurationName] == nil {
			routeConfiguration := rds.CreateRouteConfigurationForAPIListener(app.RouteConfigurationName, app.ListenerName, app.PathPrefix, app.ClusterName)
			b.routeConfigurations[routeConfiguration.Name] = routeConfiguration
			if b.features.EnableFederation {
				xdstpRouteConfigurationName := xdstpRouteConfiguration(b.authority, app.RouteConfigurationName)
				xdstpClusterName := xdstpCluster(b.authority, app.ClusterName)
				xdstpRouteConfiguration := rds.CreateRouteConfigurationForAPIListener(xdstpRouteConfigurationName, app.ListenerName, app.PathPrefix, xdstpClusterName)
				b.routeConfigurations[xdstpRouteConfiguration.Name] = xdstpRouteConfiguration
			}
		}
		if b.clusters[app.ClusterName] == nil {
			cluster, err := cds.CreateCluster(
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
				xdstpCluster, err := cds.CreateCluster(
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
		clusterLoadAssignment := eds.CreateClusterLoadAssignment(app.EDSServiceName, app.Port, b.nodeHash, b.localityPriorityMapper, b.endpointsByCluster[endpointsByClusterKey])
		b.clusterLoadAssignments[clusterLoadAssignment.ClusterName] = clusterLoadAssignment
		if b.features.EnableFederation {
			xdstpEDSServiceName := xdstpEdsService(b.authority, app.EDSServiceName)
			xdstpClusterLoadAssignment := eds.CreateClusterLoadAssignment(xdstpEDSServiceName, app.Port, b.nodeHash, b.localityPriorityMapper, b.endpointsByCluster[endpointsByClusterKey])
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

// AddGRPCServerListenerAddresses adds server listeners and associated route
// configurations with the provided IP addresses and ports to the snapshot.
func (b *SnapshotBuilder) AddGRPCServerListenerAddresses(addresses []EndpointAddress) *SnapshotBuilder {
	for _, address := range addresses {
		b.grpcServerListenerAddresses[address] = true
	}
	return b
}

// Build adds the server listeners and route configuration for the node hash, and then builds the snapshot.
func (b *SnapshotBuilder) Build() (cachev3.ResourceSnapshot, error) {
	for address := range b.grpcServerListenerAddresses {
		serverListener, err := lds.CreateGRPCServerListener(address.Host, address.Port, b.features.EnableDataPlaneTLS, b.features.RequireDataPlaneClientCerts, b.features.EnableRBAC)
		if err != nil {
			return nil, fmt.Errorf("could not create LDS server Listener for address %s:%d: %w", address.Host, address.Port, err)
		}
		b.listeners[serverListener.Name] = serverListener
	}
	if len(b.grpcServerListenerAddresses) > 0 {
		routeConfigurationForGRPCServerListener, err := rds.CreateRouteConfigurationForGRPCServerListener(b.features.EnableRBAC)
		if err != nil {
			return nil, fmt.Errorf("could not create RDS RouteConfiguration for LDS server Listener: %w", err)
		}
		b.routeConfigurations[routeConfigurationForGRPCServerListener.Name] = routeConfigurationForGRPCServerListener
	}

	// Envoy proxies will not accept the gRPC server Listeners, because all the routes in their RouteConfigurations
	// specify `NonForwardingAction` as the action.
	// Envoy proxies will also not accept the API Listeners created for gRPC clients, because Envoy proxies can only
	// have at most one API Listener defined, and that API Listener must be a static resource (not fetched via xDS).
	// TODO: Add Envoy HTTPS Listener with gRPC-JSON transcoding and gRPC HTTP/1.1 bridge.
	// https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_json_transcoder_filter
	// https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_http1_bridge_filter
	envoyGRPCListener, err := lds.CreateEnvoyGRPCListener(50051, true)
	if err != nil {
		return nil, fmt.Errorf("could not create LDS Listener for Envoy proxy receiving gRPC requests: %w", err)
	}
	b.listeners[envoyGRPCListener.Name] = envoyGRPCListener
	var clusterNames []string
	for clusterName := range b.clusters {
		clusterNames = append(clusterNames, clusterName)
	}
	routeConfigurationForEnvoyGRPCListener, err := rds.CreateRouteConfigurationForEnvoyGRPCListener(clusterNames)
	if err != nil {
		return nil, fmt.Errorf("could not create RDS RouteConfiguration for Envoy proxy gRPC LDS Listener: %w", err)
	}
	b.routeConfigurations[routeConfigurationForEnvoyGRPCListener.Name] = routeConfigurationForEnvoyGRPCListener

	listenerResources := make([]types.Resource, len(b.listeners))
	i := 0
	for _, listener := range b.listeners {
		listenerResources[i] = listener
		i++
	}
	routeConfigurationResources := make([]types.Resource, len(b.routeConfigurations))
	j := 0
	for _, routeConfiguration := range b.routeConfigurations {
		routeConfigurationResources[j] = routeConfiguration
		j++
	}
	clusterResources := make([]types.Resource, len(b.clusters))
	k := 0
	for _, cluster := range b.clusters {
		clusterResources[k] = cluster
		k++
	}
	clusterLoadAssignmentResources := make([]types.Resource, len(b.clusterLoadAssignments))
	l := 0
	for _, clusterLoadAssignment := range b.clusterLoadAssignments {
		clusterLoadAssignmentResources[l] = clusterLoadAssignment
		l++
	}

	version := strconv.FormatInt(time.Now().UnixNano(), 10)
	return cachev3.NewSnapshot(version, map[resource.Type][]types.Resource{
		resource.ListenerType: listenerResources,
		resource.RouteType:    routeConfigurationResources,
		resource.ClusterType:  clusterResources,
		resource.EndpointType: clusterLoadAssignmentResources,
	})
}
