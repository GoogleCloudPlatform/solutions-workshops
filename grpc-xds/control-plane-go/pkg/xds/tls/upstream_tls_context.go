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

	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
)

// CreateUpstreamTLSContext configures:
// 1. gRPC client TLS certificate provider
// 2. Envoy static secret name for TLS certificates and private keys
// 3. Certificate authorities (CAs) to validate gRPC server certificates, including server authorization.
// Important: Assumes that the client application k8s Service account name matches the application name!
func CreateUpstreamTLSContext(namespace string, serviceAccountName string, requireClientCerts bool) *tlsv3.UpstreamTlsContext {
	//goland:noinspection ALL
	upstreamTLSContext := tlsv3.UpstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			// AlpnProtocols is set by Traffic Director, but ignored by gRPC xDS according to gRFC A29.
			AlpnProtocols: []string{"h2"},
			// Validate gRPC server certificates:
			ValidationContextType: &tlsv3.CommonTlsContext_CombinedValidationContext{
				CombinedValidationContext: &tlsv3.CommonTlsContext_CombinedCertificateValidationContext{
					// Validate gRPC server certificates for gRPC clients:
					DefaultValidationContext: &tlsv3.CertificateValidationContext{
						CaCertificateProviderInstance: &tlsv3.CertificateProviderPluginInstance{
							InstanceName: certificateProviderInstanceName,
							// Using the same certificate name value as Traffic Director,
							// but the certificate name is ignored by gRPC, see gRFC A29.
							CertificateName: "ROOTCA",
						},
						// Server authorization (SAN checks):
						// gRPC-Java as of v1.64.0 does not work correctly with
						// `match_typed_subject_alt_names`, using deprecated
						// `match_subject_alt_names` instead, for now.
						MatchSubjectAltNames: []*matcherv3.StringMatcher{
							{
								MatchPattern: &matcherv3.StringMatcher_SafeRegex{
									SafeRegex: &matcherv3.RegexMatcher{
										Regex: fmt.Sprintf("spiffe://[^/]+/ns/%s/sa/%s", namespace, serviceAccountName),
									},
								},
							},
						},
					},
					// Validate server certificates for Envoy proxy clients:
					ValidationContextSdsSecretConfig: &tlsv3.SdsSecretConfig{
						Name: "upstream_validation", // Match the name in Envoy static_resources.secrets
					},
				},
			},
		},
	}

	if requireClientCerts {
		// Send client certificate in TLS handshake for gRPC clients:
		upstreamTLSContext.CommonTlsContext.TlsCertificateProviderInstance = &tlsv3.CertificateProviderPluginInstance{
			InstanceName: certificateProviderInstanceName,
			// Using the same certificate name value as Traffic Director, but the
			// certificate name is ignored by gRPC according to gRFC A29.
			CertificateName: "DEFAULT",
		}
		// Send client certificate in TLS handshake for Envoy proxy clients:
		upstreamTLSContext.CommonTlsContext.TlsCertificateSdsSecretConfigs = []*tlsv3.SdsSecretConfig{
			{
				Name: "upstream_cert", // Match the name in Envoy static_resources.secrets
			},
		}
	}

	return &upstreamTLSContext
}
