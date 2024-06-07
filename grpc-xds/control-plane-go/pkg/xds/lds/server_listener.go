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

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	http_connection_managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	envoyHTTPConnectionManagerName = "envoy.http_connection_manager"
	EnvoyTransportSocketsTLSName   = "envoy.transport_sockets.tls"
	// TLSCertificateProviderInstanceName is used in the `[Down|Up]streamTlsContext`s.
	// Using the same name as the `traffic-director-grpc-bootstrap` tool, but this is not important.
	// https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/2a9cf4614b56ec085c391a12f4cc53defaa575ac/main.go#L276
	TLSCertificateProviderInstanceName = "google_cloud_private_spiffe"
)

// createServerListener returns an LDS Listener that can be used for
// gRPC servers and Envoy proxy instances.
func createServerListener(listenerName string, host string, port uint32, httpConnectionManager *http_connection_managerv3.HttpConnectionManager, enableTLS bool, requireClientCerts bool) (*listenerv3.Listener, error) {
	anyWrappedHTTPConnectionManager, err := anypb.New(httpConnectionManager)
	if err != nil {
		return nil, fmt.Errorf("could not marshall HttpConnectionManager +%v into Any instance: %w", httpConnectionManager, err)
	}

	serverListener := listenerv3.Listener{
		Name: listenerName,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address: host,
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: port,
					},
					Protocol: corev3.SocketAddress_TCP,
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
		downstreamTLSContext := createDownstreamTLSContext(requireClientCerts)
		anyWrappedDownstreamTLSContext, err := anypb.New(downstreamTLSContext)
		if err != nil {
			return nil, fmt.Errorf("could not marshall DownstreamTlsContext %+v into Any instance: %w", downstreamTLSContext, err)
		}
		// Assume that HttpConnectionManager is the first (and only) filter in the Listener's filter chain:
		serverListener.FilterChains[0].TransportSocket = &corev3.TransportSocket{
			Name: EnvoyTransportSocketsTLSName,
			ConfigType: &corev3.TransportSocket_TypedConfig{
				TypedConfig: anyWrappedDownstreamTLSContext,
			},
		}
	}
	return &serverListener, nil
}

// createDownstreamTLSContext configures:
// 1. gRPC server TLS certificate provider
// 2. Envoy static secret name for TLS certificates and private keys
// 3. Certificate authorities (CAs) to validate gRPC client certificates.
func createDownstreamTLSContext(requireClientCerts bool) *tlsv3.DownstreamTlsContext {
	downstreamTLSContext := tlsv3.DownstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			// AlpnProtocols is ignored by gRPC xDS according to gRFC A29, but Envoy wants it.
			AlpnProtocols: []string{"h2"},
			// Set server certificate for gRPC:
			TlsCertificateProviderInstance: &tlsv3.CertificateProviderPluginInstance{
				InstanceName: TLSCertificateProviderInstanceName,
				// Using the same certificate name value as Traffic Director, but the
				// certificate name is ignored by gRPC according to gRFC A29.
				CertificateName: "DEFAULT",
			},
			// Set server certificate for Envoy:
			TlsCertificateSdsSecretConfigs: []*tlsv3.SdsSecretConfig{
				{
					Name: "downstream_cert", // Match the name in Envoy static_resources.secrets
				},
			},
		},
	}

	if requireClientCerts {
		// `require_client_certificate: true` requires a `validation_context`.
		downstreamTLSContext.RequireClientCertificate = wrapperspb.Bool(true)
		// Validate client certificates:
		// gRFC A29 specifies to use either `validation_context` or
		// `combined_validation_context.default_validation_context`.
		downstreamTLSContext.CommonTlsContext.ValidationContextType = &tlsv3.CommonTlsContext_CombinedValidationContext{
			CombinedValidationContext: &tlsv3.CommonTlsContext_CombinedCertificateValidationContext{
				// gRPC client config using xDS certificate provider framework:
				DefaultValidationContext: &tlsv3.CertificateValidationContext{
					CaCertificateProviderInstance: &tlsv3.CertificateProviderPluginInstance{
						InstanceName: TLSCertificateProviderInstanceName,
						// Using the same certificate name value as Traffic Director,
						// but the certificate name is ignored by gRPC, see gRFC A29.
						CertificateName: "ROOTCA",
					},
				},
				// Envoy config using static resources, see:
				// https://www.envoyproxy.io/docs/envoy/latest/configuration/security/secret
				ValidationContextSdsSecretConfig: &tlsv3.SdsSecretConfig{
					Name: "downstream_validation", // Match the name in Envoy static_resources.secrets
				},
			},
		}
	}

	return &downstreamTLSContext
}
