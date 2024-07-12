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
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// CreateDownstreamTLSContext configures:
// 1. gRPC server TLS certificate provider
// 2. Envoy static secret name for TLS certificates and private keys
// 3. Certificate authorities (CAs) to validate gRPC client certificates.
func CreateDownstreamTLSContext(requireClientCerts bool) *tlsv3.DownstreamTlsContext {
	downstreamTLSContext := tlsv3.DownstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			// AlpnProtocols is ignored by gRPC xDS according to gRFC A29, but Envoy wants it.
			AlpnProtocols: []string{"h2"},
			// Set server certificate for gRPC servers:
			TlsCertificateProviderInstance: &tlsv3.CertificateProviderPluginInstance{
				InstanceName: certificateProviderInstanceName,
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
						InstanceName: certificateProviderInstanceName,
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
