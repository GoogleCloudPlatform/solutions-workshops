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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/tls/certprovider"
	"google.golang.org/grpc/credentials/tls/certprovider/pemfile"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/security/advancedtls"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/informers"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/interceptors"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/logging"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/eds"
)

// gRPC configuration based on https://github.com/envoyproxy/go-control-plane/blob/v0.11.1/internal/example/server.go
const (
	grpcKeepaliveTime        = 30 * time.Second
	grpcKeepaliveTimeout     = 5 * time.Second
	grpcKeepaliveMinTime     = 30 * time.Second
	grpcMaxConcurrentStreams = 1000000
)

type transportCredentials struct {
	credentials.TransportCredentials
	providers []certprovider.Provider
}

// Close cleans up resources used by the credentials.
func (c *transportCredentials) Close() {
	for _, provider := range c.providers {
		if provider != nil {
			provider.Close()
		}
	}
}

func Run(ctx context.Context, servingPort int, healthPort int, kubecontexts []informers.Kubecontext, xdsFeatures *xds.Features, authority string) error {
	logger := logging.FromContext(ctx)
	serverCredentials, err := createServerCredentials(logger, xdsFeatures)
	if err != nil {
		return fmt.Errorf("could not create server-side transport credentials: %w", err)
	}
	defer serverCredentials.Close()

	grpcOptions := serverOptions(logger, serverCredentials)
	server := grpc.NewServer(grpcOptions...)
	healthGRPCServer := grpc.NewServer()
	healthServer := health.NewServer()
	addServerStopBehavior(ctx, logger, server, healthGRPCServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	healthpb.RegisterHealthServer(healthGRPCServer, healthServer)

	cleanup, err := registerAdminServers(server, healthGRPCServer)
	if err != nil {
		return fmt.Errorf("could not register gRPC Channelz and CSDS admin services: %w", err)
	}
	defer cleanup()

	reflection.Register(server)
	reflection.Register(healthGRPCServer)

	xdsCache := xds.NewSnapshotCache(ctx, true, xds.ZoneHash{}, eds.LocalityPriorityByZone{}, xdsFeatures, authority)
	xdsServer := serverv3.NewServer(ctx, xdsCache, xdsServerCallbackFuncs(logger))

	registerXDSServices(server, xdsServer)

	if err := createInformers(ctx, logger, kubecontexts, xdsCache); err != nil {
		return fmt.Errorf("could not create Kubernetes informer managers: %w", err)
	}

	tcpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", servingPort))
	if err != nil {
		return fmt.Errorf("could not create TCP listener on port=%d: %w", servingPort, err)
	}
	healthTCPListener, err := net.Listen("tcp", fmt.Sprintf(":%d", healthPort))
	if err != nil {
		return fmt.Errorf("could not create TCP listener on port=%d: %w", healthPort, err)
	}
	logger.V(1).Info("xDS control plane management server listening", "port", servingPort, "healthPort", healthPort)
	go func() {
		err := server.Serve(tcpListener)
		if err != nil {
			healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		}
	}()
	return healthGRPCServer.Serve(healthTCPListener)
}

func registerAdminServers(servingGRPCServer *grpc.Server, healthGRPCServer *grpc.Server) (func(), error) {
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

func xdsServerCallbackFuncs(logger logr.Logger) *serverv3.CallbackFuncs {
	return &serverv3.CallbackFuncs{
		StreamRequestFunc: func(streamID int64, request *discoveryv3.DiscoveryRequest) error {
			logger.Info("StreamRequest", "streamID", streamID, "type", request.GetTypeUrl(), "resourceNames", request.ResourceNames)
			return nil
		},
		StreamResponseFunc: func(_ context.Context, streamID int64, _ *discoveryv3.DiscoveryRequest, response *discoveryv3.DiscoveryResponse) {
			protoMarshalOptions := protojson.MarshalOptions{
				Multiline:    true,
				Indent:       "  ",
				AllowPartial: true,
			}
			for _, anyResource := range response.Resources {
				if anyResource == nil {
					continue
				}
				protoResource, err := anyResource.UnmarshalNew()
				if err != nil {
					logger.Error(err, "StreamResponse: could not unmarshall Any message")
					continue
				}
				jsonResourceBytes, err := protoMarshalOptions.Marshal(protoResource)
				if err != nil {
					logger.Error(err, "StreamResponse: could not marshall proto message to JSON")
					continue
				}
				// Logging each resource instead of a slice of resources, to take advantage of multi-line logging,
				// which is helpful for development and exploration.
				logger.Info("StreamResponse", "streamID", streamID, "type", response.GetTypeUrl(), "resource", string(jsonResourceBytes))
			}
		},
	}
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

func createInformers(ctx context.Context, logger logr.Logger, kubecontexts []informers.Kubecontext, xdsCache *xds.SnapshotCache) error {
	for _, kubecontext := range kubecontexts {
		informerManager, err := informers.NewManager(ctx, kubecontext.Context, xdsCache)
		if err != nil {
			return fmt.Errorf("could not create Kubernetes informer manager for context=%s: %w", kubecontext.Context, err)
		}
		for _, informer := range kubecontext.Informers {
			if err := informerManager.AddEndpointSliceInformer(ctx, logger, informer); err != nil {
				return fmt.Errorf("could not create Kubernetes informer for context=%s for %+v: %w", kubecontext.Context, informer, err)
			}
		}
	}
	return nil
}

// serverOptions sets gRPC server options.
//
// gRPC golang library sets a very small upper bound for the number gRPC/h2
// streams over a single TCP connection. If a proxy multiplexes requests over
// a single connection to the management server, then it might lead to
// availability problems.
// Keepalive timeouts based on connection_keepalive parameter https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/examples#dynamic
// Source: https://github.com/envoyproxy/go-control-plane/blob/v0.11.1/internal/example/server.go#L67
func serverOptions(logger logr.Logger, transportCredentials credentials.TransportCredentials) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainStreamInterceptor(interceptors.StreamServerLogging(logger)),
		grpc.ChainUnaryInterceptor(interceptors.UnaryServerLogging(logger)),
		grpc.Creds(transportCredentials),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcKeepaliveMinTime,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    grpcKeepaliveTime,
			Timeout: grpcKeepaliveTimeout,
		}),
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
	}
}

func createServerCredentials(logger logr.Logger, xdsFeatures *xds.Features) (*transportCredentials, error) {
	if !xdsFeatures.EnableControlPlaneTLS {
		logger.V(2).Info("using insecure credentials for the control plane server")
		return &transportCredentials{
			TransportCredentials: insecure.NewCredentials(),
		}, nil
	}
	logger.V(2).Info("using mTLS with automatic certificate reloading for the control plane server")
	identityOptions := pemfile.Options{
		CertFile:        "/var/run/secrets/workload-spiffe-credentials/certificates.pem",
		KeyFile:         "/var/run/secrets/workload-spiffe-credentials/private_key.pem",
		RefreshDuration: 600 * time.Second,
	}
	identityProvider, err := pemfile.NewProvider(identityOptions)
	if err != nil {
		return nil, fmt.Errorf("could not create a new certificate provider for identityOptions=%+v: %w", identityOptions, err)
	}
	providers := []certprovider.Provider{identityProvider}

	options := &advancedtls.Options{
		IdentityOptions: advancedtls.IdentityCertificateOptions{
			IdentityProvider: identityProvider,
		},
		AdditionalPeerVerification: func(params *advancedtls.HandshakeVerificationInfo) (*advancedtls.PostHandshakeVerificationResults, error) {
			// Not actually checking anything, just logging the client's SPIFFE ID.
			// SPIFFE certificates must have exactly one URI SAN.
			if len(params.Leaf.URIs) == 1 && params.Leaf.URIs[0] != nil {
				logger.V(2).Info("Client TLS certificate", "spiffeID", *params.Leaf.URIs[0])
			}
			return &advancedtls.PostHandshakeVerificationResults{}, nil
		},
		RequireClientCert: false,
		VerificationType:  advancedtls.CertVerification,
	}

	if xdsFeatures.RequireControlPlaneClientCerts {
		rootOptions := pemfile.Options{
			RootFile:        "/var/run/secrets/workload-spiffe-credentials/ca_certificates.pem",
			RefreshDuration: 600 * time.Second,
		}
		rootProvider, err := pemfile.NewProvider(rootOptions)
		if err != nil {
			return nil, fmt.Errorf("could not create a new certificate provider for rootOptions=%+v: %w", rootOptions, err)
		}
		providers = append(providers, rootProvider)
		options.RootOptions = advancedtls.RootCertificateOptions{
			RootProvider: rootProvider,
		}
		options.RequireClientCert = true
	}
	logger.Info("advancedtls", "options", options)
	serverCredentials, err := advancedtls.NewServerCreds(options)
	if err != nil {
		return nil, fmt.Errorf("could not create server credentials from options %+v: %w", options, err)
	}
	return &transportCredentials{
		TransportCredentials: serverCredentials,
		providers:            providers,
	}, err
}

func addServerStopBehavior(ctx context.Context, logger logr.Logger, servingGRPCServer *grpc.Server, healthGRPCServer *grpc.Server, healthServer *health.Server) {
	go func() {
		<-ctx.Done()
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		stopped := make(chan struct{})
		go func() {
			logger.Info("Attempting to gracefully stop the xDS management server")
			servingGRPCServer.GracefulStop()
			close(stopped)
		}()
		t := time.NewTimer(5 * time.Second)
		select {
		case <-t.C:
			logger.Info("Stopping the xDS management server immediately")
			servingGRPCServer.Stop()
			healthGRPCServer.Stop()
		case <-stopped:
			t.Stop()
		}
	}()
}
