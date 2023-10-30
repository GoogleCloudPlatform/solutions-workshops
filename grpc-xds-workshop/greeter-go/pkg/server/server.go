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
	"google.golang.org/grpc/credentials/insecure"
	xdscredentials "google.golang.org/grpc/credentials/xds"
	helloworldpb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/xds"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/greeter-go/pkg/greeter"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/greeter-go/pkg/interceptors"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/greeter-go/pkg/logging"
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
	Port              int
	GreeterName       string
	NextHop           string
	UseXDS            bool
	UseXDSCredentials bool
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
	grpcOptions, err := serverOptions(logger, c.UseXDSCredentials)
	if err != nil {
		return fmt.Errorf("could not set gRPC server options: %w", err)
	}

	healthServer := health.NewServer()
	grpcServer, err := newGRPCServer(ctx, logger, c.UseXDS, healthServer, grpcOptions...)
	if err != nil {
		return fmt.Errorf("could not create the gRPC server: %w", err)
	}

	var greeterService helloworldpb.GreeterServer
	if c.NextHop == "" {
		logger.V(1).Info("Adding leaf Greeter service, as NEXT_HOP is not provided")
		greeterService = greeter.NewLeafService(ctx, c.GreeterName)
	} else {
		logger.V(1).Info("Adding intermediary Greeter service", "NEXT_HOP", c.NextHop)
		greeterClient, err := greeter.NewClient(ctx, c.NextHop, c.UseXDSCredentials)
		if err != nil {
			return fmt.Errorf("could not create greeter client %w", err)
		}
		greeterService = greeter.NewIntermediaryService(ctx, c.GreeterName, greeterClient)
	}
	helloworldpb.RegisterGreeterServer(grpcServer, greeterService)

	// TODO: Start separate server for health and admin services.
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(helloworldpb.Greeter_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	if c.UseXDS {
		// Using xDS, so registering Channelz and CSDS
		cleanup, err := admin.Register(grpcServer)
		if err != nil {
			return fmt.Errorf("could not register Channelz and CSDS admin services: %w", err)
		}
		defer func() {
			logger.V(2).Info("Cleaning up admin services as the server is stopping")
			cleanup()
		}()
	} else {
		// Not using xDS, so only registering Channelz
		channelzservice.RegisterChannelzServiceToServer(grpcServer)
	}
	reflection.Register(grpcServer)

	tcpListener, err := net.Listen("tcp4", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		return fmt.Errorf("could not create TCP listener on port=%d: %w", c.Port, err)
	}
	logger.V(1).Info("Greeter service listening", "port", c.Port, "nextHop", c.NextHop)
	return grpcServer.Serve(tcpListener)
}

func serverOptions(logger logr.Logger, useXDSCredentials bool) ([]grpc.ServerOption, error) {
	// https://github.com/grpc/grpc-go/blob/v1.59.0/xds/server.go#L145
	serverCredentials := insecure.NewCredentials()
	if useXDSCredentials {
		logger.V(1).Info("Using xDS server-side credentials, with insecure as fallback")
		var err error
		if serverCredentials, err = xdscredentials.NewServerCredentials(xdscredentials.ServerOptions{FallbackCreds: insecure.NewCredentials()}); err != nil {
			return nil, fmt.Errorf("could not create server-side transport credentials for xDS: %w", err)
		}
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

func newGRPCServer(ctx context.Context, logger logr.Logger, useXDS bool, healthServer *health.Server, opts ...grpc.ServerOption) (grpcserver, error) {
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
	addServerStopBehavior(ctx, logger, server, healthServer)
	return server, nil
}

func addServerStopBehavior(ctx context.Context, logger logr.Logger, server grpcserver, healthServer *health.Server) {
	go func(s grpcserver) {
		<-ctx.Done()
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus(helloworldpb.Greeter_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
		stopped := make(chan struct{})
		go func() {
			logger.Info("Attempting to gracefully stop the gRPC server")
			s.GracefulStop()
			close(stopped)
		}()
		t := time.NewTimer(5 * time.Second)
		select {
		case <-t.C:
			logger.Info("Stopping the gRPC server immediately")
			s.Stop()
		case <-stopped:
			t.Stop()
		}
	}(server)
}
