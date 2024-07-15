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

import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CertificateProviderPluginInstance;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CertificateValidationContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CommonTlsContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.CommonTlsContext.CombinedCertificateValidationContext;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.SdsSecretConfig;
import io.envoyproxy.envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext;
import io.envoyproxy.envoy.type.matcher.v3.RegexMatcher;
import io.envoyproxy.envoy.type.matcher.v3.StringMatcher;
import org.jetbrains.annotations.NotNull;

/** Builds UpstreamTlsContexts with client TLS config for CDS Clusters. */
public class UpstreamTlsContextBuilder {

  /** xDS certificate provider instance for gRPC client certificate. */
  private String clientCertificateProviderInstanceName =
      TransportSocketBuilder.TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME;

  /**
   * Certificate name to use with xDS certificate provider instance for gRPC client certificate.
   *
   * <p>The initial value is the same as Traffic Director uses, but the certificate name is ignored
   * by gRPC according to gRFC A29.
   */
  private String clientCertificateName = "DEFAULT";

  /** Envoy static secret name of TLS client certificate for Envoy proxies. */
  private String clientCertificateSdsSecretConfigName = "upstream_cert";

  /** xDS certificate provider instance for gRPC clients when validating upstream certificates. */
  private String validationCertificateProviderInstanceName =
      TransportSocketBuilder.TLS_CERTIFICATE_PROVIDER_INSTANCE_NAME;

  /**
   * Certificate name to use with xDS certificate provider instance for gRPC clients validating
   * upstream certificates.
   *
   * <p>The initial value is the same as Traffic Director uses, but the certificate name is ignored
   * by gRPC according to gRFC A29.
   */
  private String validationCertificateName = "ROOTCA";

  /** Envoy static secret name of CA certificate bundle for validating upstream certificates. */
  private String validationContextSdsSecretConfigName = "upstream_validation";

  private boolean enableServerAuthorization;
  private String trustDomain;
  private String namespace;
  private String serviceAccount;
  private boolean requireClientCerts;

  public UpstreamTlsContextBuilder() {}

  /**
   * Use if clients should authorize servers according to <a
   * href="https://github.com/grpc/proposal/blob/deaf1bcf248d1e48e83c470b00930cbd363fab6d/A29-xds-tls-security.md#server-authorization-aka-subject-alt-name-checks">gRFC
   * A29</a>.
   *
   * <p>This creates a SAN matcher in the UpstreamTLSContext that matches the SPIFFE ID in the
   * server's certificate against the pattern <code>
   * spiffe://[trustDomain]/ns/[namespace]/sa/[serviceAccount]</code>.
   *
   * @param trustDomain the SPIFFE trust domain to match, use <code>&quot;[^/]+&quot;</code> to
   *     match any trust domain
   * @param namespace the k8s namespace to match, use <code>&quot;[^/]+&quot;</code> to match any
   *     namespace
   * @param serviceAccount k8s service account name to match, use <code>&quot;.+&quot;</code> to
   *     match any service account
   * @return the Cluster builder
   */
  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public UpstreamTlsContextBuilder withServerAuthorization(
      @NotNull String trustDomain, @NotNull String namespace, @NotNull String serviceAccount) {
    if (trustDomain.isBlank() || namespace.isBlank() || serviceAccount.isBlank()) {
      throw new IllegalArgumentException(
          "UpstreamTlsContext trustDomain, namespace, and serviceAccount must all be"
              + " non-blank when enabling server authorization");
    }
    this.enableServerAuthorization = true;
    this.trustDomain = trustDomain;
    this.namespace = namespace;
    this.serviceAccount = serviceAccount;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public UpstreamTlsContextBuilder withRequireClientCerts(boolean requireClientCerts) {
    this.requireClientCerts = requireClientCerts;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public UpstreamTlsContextBuilder withClientCertificateProviderInstanceName(
      @NotNull String clientCertificateProviderInstanceName) {
    this.clientCertificateProviderInstanceName = clientCertificateProviderInstanceName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public UpstreamTlsContextBuilder withClientCertificateName(
      @NotNull String clientCertificateName) {
    this.clientCertificateName = clientCertificateName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public UpstreamTlsContextBuilder withClientCertificateSdsSecretConfigName(
      @NotNull String clientCertificateSdsSecretConfigName) {
    this.clientCertificateSdsSecretConfigName = clientCertificateSdsSecretConfigName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public UpstreamTlsContextBuilder withValidationCertificateProviderInstanceName(
      @NotNull String validationCertificateProviderInstanceName) {
    this.validationCertificateProviderInstanceName = validationCertificateProviderInstanceName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public UpstreamTlsContextBuilder withValidationCertificateName(
      @NotNull String validationCertificateName) {
    this.validationCertificateName = validationCertificateName;
    return this;
  }

  @SuppressWarnings({"unused", "UnusedReturnValue"})
  @NotNull
  public UpstreamTlsContextBuilder withValidationContextSdsSecretConfigName(
      @NotNull String validationContextSdsSecretConfigName) {
    this.validationContextSdsSecretConfigName = validationContextSdsSecretConfigName;
    return this;
  }

  /** Creates the UpstreamTlsContext. */
  @NotNull
  public UpstreamTlsContext build() {
    // Validate upstream (server) certificates:
    var certificateValidationContextBuilder =
        CertificateValidationContext.newBuilder()
            // CA certificates for gRPC clients to validate upstream certificates:
            .setCaCertificateProviderInstance(
                CertificateProviderPluginInstance.newBuilder()
                    .setInstanceName(validationCertificateProviderInstanceName)
                    .setCertificateName(validationCertificateName)
                    .build());

    if (enableServerAuthorization) {
      // TODO: MatchSubjectAltNames to be replaced by MatchTypedSubjectAltNames
      //noinspection deprecation
      certificateValidationContextBuilder.addMatchSubjectAltNames(
          StringMatcher.newBuilder()
              .setSafeRegex(
                  RegexMatcher.newBuilder()
                      .setRegex(
                          "spiffe://%s/ns/%s/sa/%s"
                              .formatted(trustDomain, namespace, serviceAccount))
                      .build())
              .build());
    }

    // CA certificates for Envoy proxy clients to validate upstream certificates:
    var sdsSecretConfigBuilder =
        SdsSecretConfig.newBuilder()
            // Match the name in Envoy static_resources.secrets
            .setName(validationContextSdsSecretConfigName);

    var commonTlsContextBuilder =
        CommonTlsContext.newBuilder()
            // AlpnProtocols is ignored by gRPC according to gRFC A29, but Envoy wants it.
            .addAlpnProtocols("h2")
            // Validate upstream (server) certificate:
            .setCombinedValidationContext(
                CombinedCertificateValidationContext.newBuilder()
                    .setDefaultValidationContext(certificateValidationContextBuilder.build())
                    .setValidationContextSdsSecretConfig(sdsSecretConfigBuilder.build())
                    .build());

    if (requireClientCerts) {
      // Client certificate for gRPC clients:
      commonTlsContextBuilder.setTlsCertificateProviderInstance(
          CertificateProviderPluginInstance.newBuilder()
              .setInstanceName(clientCertificateProviderInstanceName)
              .setCertificateName(clientCertificateName)
              .build());
      // Client certificate for Envoy proxy clients:
      commonTlsContextBuilder.addTlsCertificateSdsSecretConfigs(
          SdsSecretConfig.newBuilder().setName(clientCertificateSdsSecretConfigName).build());
    }

    return UpstreamTlsContext.newBuilder()
        .setCommonTlsContext(commonTlsContextBuilder.build())
        .build();
  }
}
