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

package greeter

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	xdscredentials "google.golang.org/grpc/credentials/xds"
	helloworldpb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/keepalive"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/interceptors"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/logging"
)

const (
	grpcClientDialTimeout      = 10 * time.Second
	grpcClientKeepaliveTime    = 30 * time.Second
	grpcClientKeepaliveTimeout = 5 * time.Second
	grpcClientIdleTimeout      = math.MaxInt64 // good idea?
)

type Client struct {
	logger  logr.Logger
	nextHop string
	client  helloworldpb.GreeterClient
}

func NewClient(ctx context.Context, nextHop string, useXDSCredentials bool) (*Client, error) {
	logger := logging.FromContext(ctx)
	dialOpts, err := dialOptions(logger, useXDSCredentials)
	if err != nil {
		return nil, fmt.Errorf("could not configure greeter client connection dial options: %w", err)
	}
	dialCtx, dialCancel := context.WithTimeout(ctx, grpcClientDialTimeout)
	defer dialCancel()
	clientConn, err := grpc.DialContext(dialCtx, nextHop, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not create a virtual connection to target=%s: %w", nextHop, err)
	}
	addClientConnectionCloseBehavior(ctx, logger, clientConn)
	return &Client{
		client:  helloworldpb.NewGreeterClient(clientConn),
		logger:  logger,
		nextHop: nextHop,
	}, nil
}

func (c *Client) SayHello(requestCtx context.Context, name string) (string, error) {
	resp, err := c.client.SayHello(requestCtx, &helloworldpb.HelloRequest{Name: name}, grpc.WaitForReady(true))
	if err != nil {
		return "", fmt.Errorf("could not greet name=%s at target=%s: %w", name, c.nextHop, err)
	}
	return resp.GetMessage(), nil
}

// dialOptions sets parameters for client connection establishment.
func dialOptions(logger logr.Logger, useXDSCredentials bool) ([]grpc.DialOption, error) {
	clientCredentials := insecure.NewCredentials()
	if useXDSCredentials {
		logger.V(1).Info("Using xDS client-side credentials, with insecure as fallback")
		var err error
		if clientCredentials, err = xdscredentials.NewClientCredentials(xdscredentials.ClientOptions{FallbackCreds: insecure.NewCredentials()}); err != nil {
			return nil, fmt.Errorf("could not create client-side transport credentials for xDS: %w", err)
		}
	}
	return []grpc.DialOption{
		grpc.WithChainStreamInterceptor(interceptors.StreamClientLogging(logger)),
		grpc.WithChainUnaryInterceptor(interceptors.UnaryClientLogging(logger)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                grpcClientKeepaliveTime,
			Timeout:             grpcClientKeepaliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithIdleTimeout(time.Duration(grpcClientIdleTimeout)),
		grpc.WithTransportCredentials(clientCredentials),
	}, nil
}

func addClientConnectionCloseBehavior(ctx context.Context, logger logr.Logger, clientConn *grpc.ClientConn) {
	go func(cc *grpc.ClientConn) {
		<-ctx.Done()
		logger.Info("Closing the greeter client connection")
		err := cc.Close()
		if err != nil {
			logger.Error(err, "Error when closing the greeter client connection")
		}
	}(clientConn)
}
