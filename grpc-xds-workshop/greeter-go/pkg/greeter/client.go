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
	"sync"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	xdscredentials "google.golang.org/grpc/credentials/xds"
	helloworldpb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/keepalive"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/greeter-go/pkg/interceptors"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/greeter-go/pkg/logging"
)

const (
	grpcClientDialTimeout      = 10 * time.Second
	grpcClientKeepaliveTime    = 30 * time.Second
	grpcClientKeepaliveTimeout = 5 * time.Second
	grpcClientIdleTimeout      = math.MaxInt64 // good idea?
)

type Client struct {
	mu        sync.Mutex
	clientCtx context.Context
	logger    logr.Logger
	nextHop   string
	dialOpts  []grpc.DialOption
	client    helloworldpb.GreeterClient
}

func NewClient(ctx context.Context, nextHop string, useXDSCredentials bool) (*Client, error) {
	dialOpts, err := dialOptions(logging.FromContext(ctx), useXDSCredentials)
	if err != nil {
		return nil, fmt.Errorf("could not configure greeter client connection dial options: %w", err)
	}
	return &Client{
		clientCtx: ctx,
		logger:    logging.FromContext(ctx),
		nextHop:   nextHop,
		dialOpts:  dialOpts,
	}, nil
}

func (c *Client) SayHello(requestCtx context.Context, name string) (string, error) {
	err := c.createClientIfRequired(requestCtx)
	if err != nil {
		return "", fmt.Errorf("could not create greeter client: %w", err)
	}
	resp, err := c.client.SayHello(requestCtx, &helloworldpb.HelloRequest{Name: name}, grpc.WaitForReady(true))
	if err != nil {
		return "", fmt.Errorf("could not greet name=%s at target=%s: %w", name, c.nextHop, err)
	}
	return resp.GetMessage(), nil
}

// nolint: contextcheck
func (c *Client) createClientIfRequired(requestCtx context.Context) error {
	if c.client != nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	dialCtx, dialCancel := context.WithTimeout(requestCtx, grpcClientDialTimeout)
	defer dialCancel()
	clientConn, err := grpc.DialContext(dialCtx, c.nextHop, c.dialOpts...)
	if err != nil {
		return fmt.Errorf("could not create a virtual connection to target=%s: %w", c.nextHop, err)
	}
	addClientConnectionCloseBehavior(c.clientCtx, c.logger, clientConn)
	c.client = helloworldpb.NewGreeterClient(clientConn)
	return nil
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

func addClientConnectionCloseBehavior(clientCtx context.Context, logger logr.Logger, clientConn *grpc.ClientConn) {
	go func(cc *grpc.ClientConn) {
		<-clientCtx.Done()
		logger.Info("Closing the greeter client connection")
		err := cc.Close()
		if err != nil {
			logger.Error(err, "Error when closing the greeter client connection")
		}
	}(clientConn)
}
