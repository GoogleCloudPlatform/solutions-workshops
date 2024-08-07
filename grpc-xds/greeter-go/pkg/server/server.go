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

package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/admin"
	channelzservice "google.golang.org/grpc/channelz/service"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	xdscredentials "google.golang.org/grpc/credentials/xds"
	helloworldpb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/xds"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/greeter"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/interceptors"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/logging"
)

// gRPC configuration based on https://github.com/envoyproxy/go-control-plane/blob/v0.11.1/internal/example/server.go
const (
	grpcKeepaliveTime        = 30 * time.Second
	grpcKeepaliveTimeout     = 5 * time.Second
	grpcKeepaliveMinTime     = 30 * time.Second
	grpcMaxConcurrentStreams = 1000000
)

// Config provides server parameters read from the environment.
type Config struct {
	ServingPort    int
	HealthPort     int
	HTTPHealthPort int
	GreeterName    string
	NextHop        string
	UseXDS         bool
}

// grpcserver is implemented by both grpc.Server and xds.GRPCServer.
type grpcserver interface {
	grpc.ServiceRegistrar
	reflection.ServiceInfoProvider
	Serve(net.Listener) error
	GracefulStop()
	Stop()
}

func Run(ctx context.Context, c Config) error {
	logger := logging.FromContext(ctx)
	healthServer := health.NewServer()
	serverOptions, err := configureServerOptions(logger, healthServer)
	if err != nil {
		return fmt.Errorf("could not set gRPC server options: %w", err)
	}

	servingGRPCServer, err := newGRPCServer(logger, c.UseXDS, serverOptions...)
	if err != nil {
		return fmt.Errorf("could not create the serving gRPC server: %w", err)
	}
	healthGRPCServer := grpc.NewServer() // naming is hard :-(
	addServerStopBehavior(ctx, logger, servingGRPCServer, healthGRPCServer, healthServer)

	if err := greeter.RegisterServer(ctx, logger, c.GreeterName, c.NextHop, servingGRPCServer); err != nil {
		return fmt.Errorf("could not register Greeter server: %w", err)
	}

	// Register health server on both serving and health ports
	// Set serving status for k8s startup and liveness probes:
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	// Set serving status for k8s readiness probes:
	healthServer.SetServingStatus(helloworldpb.Greeter_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	healthpb.RegisterHealthServer(servingGRPCServer, healthServer)
	healthpb.RegisterHealthServer(healthGRPCServer, healthServer)

	// Register admin services on both serving and health ports
	cleanupAdminServers, err := registerAdminServers(c.UseXDS, servingGRPCServer, healthGRPCServer)
	if err != nil {
		return fmt.Errorf("could not register admin servers: %w", err)
	}
	defer func() {
		logger.V(2).Info("Cleaning up admin servers as the gRPC server is stopping")
		cleanupAdminServers()
	}()

	// Enable reflection on both serving and health ports
	reflection.Register(servingGRPCServer)
	reflection.Register(healthGRPCServer)

	return serve(logger, c, servingGRPCServer, healthServer, healthGRPCServer)
}

func configureServerOptions(logger logr.Logger, healthServer *health.Server) ([]grpc.ServerOption, error) {
	logger.V(1).Info("Using xDS server-side credentials, with insecure as fallback")
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
		xds.ServingModeCallback(func(addr net.Addr, args xds.ServingModeChangeArgs) {
			switch args.Mode {
			case connectivity.ServingModeStarting:
				logger.Info("Attempting to connect to the xDS control plane management server")
			case connectivity.ServingModeServing:
				// Make k8s readiness probes pass.
				// TODO: Enhance this Listener so readiness probes only pass after the server Listener and RouteConfiguration resources have been ACKed.
				logger.Info("Connected to the xDS control plane management server")
				healthServer.SetServingStatus(helloworldpb.Greeter_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
			case connectivity.ServingModeNotServing:
				// Do _not_ make k8s readiness probes fail, because the greeter server can continue using the last ACKed configuration.
				logger.Error(args.Err, "Lost connection to the xDS control plane management server, using cached configuration", "xdsControlPlaneServingMode", args.Mode.String())
				healthServer.SetServingStatus(helloworldpb.Greeter_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
			}
		}),
	}, nil
}

func newGRPCServer(logger logr.Logger, useXDS bool, opts ...grpc.ServerOption) (grpcserver, error) {
	var server grpcserver
	if !useXDS {
		logger.V(1).Info("Creating a non-xDS managed server")
		server = grpc.NewServer(opts...)
	} else {
		logger.V(1).Info("Creating an xDS-managed server")
		var err error
		server, err = xds.NewGRPCServer(opts...)
		if err != nil {
			return nil, fmt.Errorf("could not create xDS-enabled gRPC server: %w", err)
		}
	}
	return server, nil
}

func addServerStopBehavior(ctx context.Context, logger logr.Logger, servingGRPCServer grpcserver, healthGRPCServer grpcserver, healthServer *health.Server) {
	go func() {
		<-ctx.Done()
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus(helloworldpb.Greeter_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
		stopped := make(chan struct{})
		go func() {
			logger.Info("Attempting to gracefully stop the gRPC server")
			servingGRPCServer.GracefulStop()
			close(stopped)
		}()
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-timer.C:
			logger.Info("Stopping the gRPC server immediately")
			servingGRPCServer.Stop()
			healthGRPCServer.Stop()
		case <-stopped:
			timer.Stop()
		}
	}()
}

func registerAdminServers(useXDS bool, servingGRPCServer grpcserver, healthGRPCServer grpcserver) (func(), error) {
	if !useXDS {
		// Not using xDS, so only registering Channelz
		channelzservice.RegisterChannelzServiceToServer(servingGRPCServer)
		channelzservice.RegisterChannelzServiceToServer(healthGRPCServer)
		return func() {}, nil
	}
	// Using xDS, so registering Channelz and CSDS
	cleanupServing, err := admin.Register(servingGRPCServer)
	if err != nil {
		return func() {}, fmt.Errorf("could not register Channelz and CSDS admin services to serving server: %w", err)
	}
	cleanupHealth, err := admin.Register(healthGRPCServer)
	if err != nil {
		return func() {}, fmt.Errorf("could not register Channelz and CSDS admin services to health server: %w", err)
	}
	return func() {
		cleanupServing()
		cleanupHealth()
	}, nil
}

func serve(logger logr.Logger, c Config, servingGRPCServer grpcserver, healthServer *health.Server, healthGRPCServer *grpc.Server) error {
	servingListener, err := net.Listen("tcp4", fmt.Sprintf(":%d", c.ServingPort))
	if err != nil {
		return fmt.Errorf("could not create TCP listener on gRPC serving port=%d: %w", c.ServingPort, err)
	}
	healthListener, err := net.Listen("tcp4", fmt.Sprintf(":%d", c.HealthPort))
	if err != nil {
		return fmt.Errorf("could not create TCP listener on gRPC health port=%d: %w", c.HealthPort, err)
	}
	httpHealthListener, err := net.Listen("tcp4", fmt.Sprintf(":%d", c.HTTPHealthPort))
	if err != nil {
		return fmt.Errorf("could not create TCP listener on HTTP health port=%d: %w", c.HealthPort, err)
	}
	logger.V(1).Info("Greeter service listening", "port", c.ServingPort, "healthPort", c.HealthPort, "httpHealthPort", c.HTTPHealthPort, "nextHop", c.NextHop)
	go func() {
		err := servingGRPCServer.Serve(servingListener)
		if err != nil {
			healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
			healthServer.SetServingStatus(helloworldpb.Greeter_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
		}
	}()
	go func() {
		listenHTTPHealth(logger, httpHealthListener, healthServer)
	}()
	return healthGRPCServer.Serve(healthListener)
}
