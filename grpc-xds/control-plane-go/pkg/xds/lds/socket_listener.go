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

package lds

import (
	"fmt"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	http_connection_managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/tls"
)

const (
	envoyHTTPConnectionManagerName = "envoy.http_connection_manager"
)

// createSocketListener returns an LDS Listener that can be used for
// gRPC servers and Envoy proxy instances.
func createSocketListener(listenerName string, host string, port uint32, httpConnectionManager *http_connection_managerv3.HttpConnectionManager, enableTLS bool, requireClientCerts bool) (*listenerv3.Listener, error) {
	anyWrappedHTTPConnectionManager, err := anypb.New(httpConnectionManager)
	if err != nil {
		return nil, fmt.Errorf("could not marshall HttpConnectionManager +%v into Any instance: %w", httpConnectionManager, err)
	}

	isIPv6 := strings.Count(host, ":") >= 2

	serverListener := listenerv3.Listener{
		Name: listenerName,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address: host,
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: port,
					},
					Protocol:   corev3.SocketAddress_TCP,
					Ipv4Compat: isIPv6,
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
		downstreamTLSContext := tls.CreateDownstreamTLSContext(requireClientCerts)
		transportSocket, err := tls.CreateTransportSocket(downstreamTLSContext)
		if err != nil {
			return nil, err
		}
		// Assume that HttpConnectionManager is the first (and only) filter in the Listener's filter chain:
		serverListener.FilterChains[0].TransportSocket = transportSocket
	}
	return &serverListener, nil
}
