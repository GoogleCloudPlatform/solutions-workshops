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

package tls

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	envoyTransportSocketsTLSName = "envoy.transport_sockets.tls"
	// certificateProviderInstanceName is used in the `[Down|Up]streamTlsContext`s.
	// Using the same name as the `traffic-director-grpc-bootstrap` tool, but this is not important.
	// https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/2a9cf4614b56ec085c391a12f4cc53defaa575ac/main.go#L276
	certificateProviderInstanceName = "google_cloud_private_spiffe"
)

// CreateTransportSocket creates a TLS transport socket for LDS Listeners and CDS Clusters.
func CreateTransportSocket(upstreamOrDownstreamTLSContext proto.Message) (*corev3.TransportSocket, error) {
	switch msg := upstreamOrDownstreamTLSContext.(type) {
	case *tlsv3.UpstreamTlsContext:
	case *tlsv3.DownstreamTlsContext:
	default:
		return nil, fmt.Errorf("expected either tlsv3.UpstreamTlsContext or tlsv3.DownstreamTlsContext, got %T", msg)
	}
	anyWrappedUpstreamTLSContext, err := anypb.New(upstreamOrDownstreamTLSContext)
	if err != nil {
		return nil, fmt.Errorf("could not marshall TLS Context +%v into Any instance: %w", upstreamOrDownstreamTLSContext, err)
	}
	return &corev3.TransportSocket{
		Name: envoyTransportSocketsTLSName,
		ConfigType: &corev3.TransportSocket_TypedConfig{
			TypedConfig: anyWrappedUpstreamTLSContext,
		},
	}, nil
}
