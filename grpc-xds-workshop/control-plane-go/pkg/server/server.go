// Copyright 2023 Google LLC
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

package server

import (
	"context"
	"fmt"
	"net"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimev3 "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretv3 "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/admin"
	"google.golang.org/grpc/credentials/insecure"
	xdscredentials "google.golang.org/grpc/credentials/xds"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/informers"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/interceptors"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/logging"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/xds"
)

// gRPC configuration based on https://github.com/envoyproxy/go-control-plane/blob/v0.11.1/internal/example/server.go
const (
	grpcKeepaliveTime        = 30 * time.Second
	grpcKeepaliveTimeout     = 5 * time.Second
	grpcKeepaliveMinTime     = 30 * time.Second
	grpcMaxConcurrentStreams = 1000000
)

func Run(ctx context.Context, port int, informerConfigs []informers.Config) error {
	logger := logging.FromContext(ctx)
	grpcOptions, err := serverOptions(logger)
	if err != nil {
		return fmt.Errorf("could not set gRPC server options: %w", err)
	}
	healthServer := health.NewServer()
	server := grpc.NewServer(grpcOptions...)
	addServerStopBehavior(ctx, logger, server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	cleanup, err := admin.Register(server)
	if err != nil {
		return fmt.Errorf("could not register gRPC Channelz and CSDS admin services: %w", err)
	}
	defer cleanup()
	reflection.Register(server)

	xdsCache := xds.NewSnapshotCache(ctx, true, xds.FixedHash{})
	xdsServer := serverv3.NewServer(ctx, xdsCache, serverv3.CallbackFuncs{
		StreamRequestFunc: func(_ int64, request *discoveryv3.DiscoveryRequest) error {
			logger.Info("StreamRequest", "type", request.GetTypeUrl(), "resourceNames", request.ResourceNames)
			return nil
		},
	})

	registerXDSServices(server, xdsServer)
	informerManager, err := informers.NewManager(ctx, xdsCache)
	if err != nil {
		return fmt.Errorf("could not create the Kubernetes informer manager: %w", err)
	}
	for _, informerConfig := range informerConfigs {
		if err := informerManager.AddEndpointSliceInformer(ctx, informerConfig); err != nil {
			return fmt.Errorf("could not create Kubernetes informer for %+v: %w", informerConfig, err)
		}
	}

	tcpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("could not create TCP listener on port=%d: %w", port, err)
	}
	logger.V(1).Info("xDS control plane management server listening", "port", port)
	return server.Serve(tcpListener)
}

func registerXDSServices(grpcServer *grpc.Server, xdsServer serverv3.Server) {
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)
	endpointv3.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)
	clusterv3.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)
	routev3.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)
	listenerv3.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)
	secretv3.RegisterSecretDiscoveryServiceServer(grpcServer, xdsServer)
	runtimev3.RegisterRuntimeDiscoveryServiceServer(grpcServer, xdsServer)
}

// serverOptions sets gRPC server options.
//
// gRPC golang library sets a very small upper bound for the number gRPC/h2
// streams over a single TCP connection. If a proxy multiplexes requests over
// a single connection to the management server, then it might lead to
// availability problems.
// Keepalive timeouts based on connection_keepalive parameter https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/examples#dynamic
// Source: https://github.com/envoyproxy/go-control-plane/blob/v0.11.1/internal/example/server.go#L67
func serverOptions(logger logr.Logger) ([]grpc.ServerOption, error) {
	serverCredentials, err := xdscredentials.NewServerCredentials(xdscredentials.ServerOptions{FallbackCreds: insecure.NewCredentials()})
	if err != nil {
		return nil, fmt.Errorf("could not create server-side transport credentials for xDS: %w", err)
	}
	return []grpc.ServerOption{
		grpc.ChainStreamInterceptor(interceptors.StreamServerLogging(logger)),
		grpc.ChainUnaryInterceptor(interceptors.UnaryServerLogging(logger)),
		grpc.Creds(serverCredentials),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcKeepaliveMinTime,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    grpcKeepaliveTime,
			Timeout: grpcKeepaliveTimeout,
		}),
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
	}, nil
}

func addServerStopBehavior(ctx context.Context, logger logr.Logger, server *grpc.Server, healthServer *health.Server) {
	go func(s *grpc.Server, h *health.Server) {
		<-ctx.Done()
		h.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		stopped := make(chan struct{})
		go func() {
			logger.Info("Attempting to gracefully stop the xDS management server")
			s.GracefulStop()
			close(stopped)
		}()
		t := time.NewTimer(5 * time.Second)
		select {
		case <-t.C:
			logger.Info("Stopping the xDS management server immediately")
			s.Stop()
		case <-stopped:
			t.Stop()
		}
	}(server, healthServer)
}
