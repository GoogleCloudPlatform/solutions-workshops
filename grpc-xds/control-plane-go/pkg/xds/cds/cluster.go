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

package cds

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/wrapperspb"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	httpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/tls"
)

const (
	envoyExtensionsUpstreamsHTTPProtocolOptions = "envoy.extensions.upstreams.http.v3.HttpProtocolOptions"
)

var (
	// TODO: Make these configurable.
	healthCheckInterval = durationpb.New(30 * time.Second)
	healthCheckTimeout  = durationpb.New(1 * time.Second)
)

// CreateCluster returns a CDS Cluster.
//
// `edsServiceName` is the resource name to request from EDS (for Clusters that use EDS).
// Typically, this is just the CDS Cluster name, but it must be a different name if the CDS
// Cluster name uses the `xdstp://` scheme for xDS federation.
//
// To enable client-side active health checking, provide a `healthCheckProtocol` value of one of
// `grpc`, `http`, or `tcp`. If the health check port is different to the serving port, provide
// the health check port number too.
//
// If the health check port is the same as the serving port, you can provide `0` as the value of
// `healthCheckPort`.
//
// `pathOrGRPCService` is the URL path for HTTP health checks, or the gRPC service name for gRPC
// health checks. It is ignored for TCP health checks.
//
// To disable client-side health checking, set `healthCheckProtocol` to an empty string.
//
// Client-side active health checks are supported by Envoy proxy, but not by gRPC clients.
// See https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/service_discovery#on-eventually-consistent-service-discovery
// and https://github.com/grpc/grpc/issues/34581
//
// TODO: Clean up too many parameters.
func CreateCluster(name string, edsServiceName string, namespace string, serviceAccountName string, healthCheckPort uint32, healthCheckProtocol string, healthCheckPathOrGRPCService string, enableTLS bool, requireClientCerts bool) (*clusterv3.Cluster, error) {
	anyWrappedHTTPProtocolOptions, err := anypb.New(&httpv3.HttpProtocolOptions{
		UpstreamProtocolOptions: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
					Http2ProtocolOptions: &corev3.Http2ProtocolOptions{},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not marshall HttpProtocolOptions into Any instance: %w", err)
	}
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
		// See https://github.com/envoyproxy/envoy/issues/11527
		// IgnoreHealthOnHostRemoval: true,
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			envoyExtensionsUpstreamsHTTPProtocolOptions: anyWrappedHTTPProtocolOptions,
		},
		// See https://github.com/envoyproxy/envoy/issues/11527
		IgnoreHealthOnHostRemoval: true,
		LbPolicy:                  clusterv3.Cluster_ROUND_ROBIN,
	}

	// Client-side active health checks. Implemented by Envoy, but not by gRPC clients.
	if healthCheckProtocol != "" {
		cluster.HealthChecks = []*corev3.HealthCheck{createHealthCheck(healthCheckProtocol, healthCheckPort, healthCheckPathOrGRPCService)}
		if healthCheckPort != 0 {
			cluster.HealthChecks[0].AltPort = wrapperspb.UInt32(healthCheckPort)
		}
	}

	if enableTLS {
		upstreamTLSContext := tls.CreateUpstreamTLSContext(namespace, serviceAccountName, requireClientCerts)
		transportSocket, err := tls.CreateTransportSocket(upstreamTLSContext)
		if err != nil {
			return nil, err
		}
		cluster.TransportSocket = transportSocket
	}

	return &cluster, nil
}

func createHealthCheck(protocol string, port uint32, pathOrGRPCService string) *corev3.HealthCheck {
	healthCheck := &corev3.HealthCheck{
		AltPort:            wrapperspb.UInt32(port),
		HealthyThreshold:   wrapperspb.UInt32(1),
		Interval:           healthCheckInterval,
		Timeout:            healthCheckTimeout,
		UnhealthyThreshold: wrapperspb.UInt32(1),
	}
	if strings.EqualFold(protocol, "grpc") {
		healthCheck.HealthChecker = &corev3.HealthCheck_GrpcHealthCheck_{
			GrpcHealthCheck: &corev3.HealthCheck_GrpcHealthCheck{
				ServiceName: pathOrGRPCService,
			},
		}
	} else if strings.EqualFold(protocol, "http") {
		healthCheck.HealthChecker = &corev3.HealthCheck_HttpHealthCheck_{
			HttpHealthCheck: &corev3.HealthCheck_HttpHealthCheck{
				Path: pathOrGRPCService,
			},
		}
	} else {
		// TCP fallback
		healthCheck.HealthChecker = &corev3.HealthCheck_TcpHealthCheck_{
			TcpHealthCheck: &corev3.HealthCheck_TcpHealthCheck{},
		}
	}
	return healthCheck
}
