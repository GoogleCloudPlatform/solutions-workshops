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

package com.google.examples.xds.controlplane.xds.tls;

import com.google.protobuf.BoolValue;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CertificateProviderPluginInstance;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CertificateValidationContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CommonTlsContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CommonTlsContext.CombinedCertificateValidationContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.SdsSecretConfig;
import org.jetbrains.annotations.NotNull;

/** Builds DownstreamTlsContexts with server TLS config for LDS socket Listeners. */
public class DownstreamTlsContextBuilder {

  /** xDS certificate provider instance for gRPC server certificate. */
  private String serverCertificateProviderInstanceName =
      TransportSocketBuilder.TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME;

  /**
   * Certificate name to use with xDS certificate provider instance for gRPC server certificate.
   *
   * <p>The initial value is the same as Traffic Director uses, but the certificate name is ignored
   * by gRPC according to gRFC A29.
   */
  private String serverCertificateName = "DEFAULT";

  /** Envoy static secret name of TLS server certificate for Envoy proxies. */
  private String serverCertificateSdsSecretConfigName = "downstream_cert";

  /**
   * Whether the Envoy proxy or gRPC server requires that clients authenticate themselves using TLS
   * client certificates.
   */
  private boolean requireClientCerts;

  /** xDS certificate provider instance for gRPC servers when validating downstream certificates. */
  private String validationCertificateProviderInstanceName =
      TransportSocketBuilder.TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME;

  /**
   * Certificate name to use with xDS certificate provider instance for gRPC servers validating
   * downstream certificates.
   *
   * <p>The initial value is the same as Traffic Director uses, but the certificate name is ignored
   * by gRPC according to gRFC A29.
   */
  private String validationCertificateName = "ROOTCA";

  /** Envoy static secret name of CA certificate bundle for validating downstream certificates. */
  private String validationContextSdsSecretConfigName = "downstream_validation";

  public DownstreamTlsContextBuilder() {}

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public DownstreamTlsContextBuilder withServerCertificateProviderInstanceName(
      @NotNull String serverCertificateProviderInstanceName) {
    this.serverCertificateProviderInstanceName = serverCertificateProviderInstanceName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public DownstreamTlsContextBuilder withServerCertificateName(
      @NotNull String serverCertificateName) {
    this.serverCertificateName = serverCertificateName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public DownstreamTlsContextBuilder withServerCertificateSdsSecretConfigName(
      @NotNull String serverCertificateSdsSecretConfigName) {
    this.serverCertificateSdsSecretConfigName = serverCertificateSdsSecretConfigName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public DownstreamTlsContextBuilder withRequireClientCerts(boolean requireClientCerts) {
    this.requireClientCerts = requireClientCerts;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public DownstreamTlsContextBuilder withValidationCaCertificateProviderInstanceName(
      @NotNull String validationCaCertificateProviderInstanceName) {
    this.validationCertificateProviderInstanceName = validationCaCertificateProviderInstanceName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public DownstreamTlsContextBuilder withValidationCaCertificateName(
      @NotNull String validationCaCertificateName) {
    this.validationCertificateName = validationCaCertificateName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public DownstreamTlsContextBuilder withValidationContextSdsSecretConfigName(
      @NotNull String validationContextSdsSecretConfigName) {
    this.validationContextSdsSecretConfigName = validationContextSdsSecretConfigName;
    return this;
  }

  /** Creates the DownstreamTlsContext. */
  @NotNull
  public DownstreamTlsContext build() {
    var commonTlsContextBuilder =
        CommonTlsContext.newBuilder()
            // AlpnProtocols is ignored by gRPC according to gRFC A29, but Envoy wants it.
            .addAlpnProtocols("h2")
            // Set server certificate for gRPC servers:
            .setTlsCertificateProviderInstance(
                CertificateProviderPluginInstance.newBuilder()
                    // https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/2a9cf4614b56ec085c391a12f4cc53defaa575ac/main.go#L276
                    .setInstanceName(serverCertificateProviderInstanceName)
                    .setCertificateName(serverCertificateName)
                    .build())
            // Set server certificate for Envoy:
            .addTlsCertificateSdsSecretConfigs(
                SdsSecretConfig.newBuilder().setName(serverCertificateSdsSecretConfigName).build());

    var downstreamTlsContextBuilder = DownstreamTlsContext.newBuilder();

    if (requireClientCerts) {
      // `require_client_certificate: true` requires a `validation_context` or.
      downstreamTlsContextBuilder.setRequireClientCertificate(BoolValue.of(true));
      // Validate client certificates:
      // gRFC A29 specifies to use either ValidationContext, or
      // CombinedValidationContext with a DefaultValidationContext:
      commonTlsContextBuilder.setCombinedValidationContext(
          CombinedCertificateValidationContext.newBuilder()
              // gRPC client config using xDS certificate provider framework:
              .setDefaultValidationContext(
                  CertificateValidationContext.newBuilder()
                      .setCaCertificateProviderInstance(
                          CertificateProviderPluginInstance.newBuilder()
                              .setInstanceName(validationCertificateProviderInstanceName)
                              // Using the same certificate name value as Traffic Director,
                              // but the certificate name is ignored by gRPC, see gRFC A29.
                              .setCertificateName(validationCertificateName)
                              .build())
                      .build())
              // Envoy config using static resources, see:
              // https://www.envoyproxy.io/docs/envoy/latest/configuration/security/secret
              .setValidationContextSdsSecretConfig(
                  SdsSecretConfig.newBuilder()
                      .setName(validationContextSdsSecretConfigName)
                      .build())
              .build());
    }

    downstreamTlsContextBuilder.setCommonTlsContext(commonTlsContextBuilder.build());

    return downstreamTlsContextBuilder.build();
  }
}
