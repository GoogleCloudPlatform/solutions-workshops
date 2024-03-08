// Copyright 2024 Google LLC
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

package com.google.examples.xds.controlplane.auth;

import com.google.auth.oauth2.GoogleCredentials;
import io.kubernetes.client.util.authenticators.Authenticator;
import java.io.IOException;
import java.time.Duration;
import java.time.Instant;
import java.util.Date;
import java.util.HashMap;
import java.util.Map;
import org.apache.logging.log4j.util.Strings;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Implements a "google" <code>auth_provider</code> for Kubernetes.
 *
 * <p>Relies on Application Default Credentials, and does not support command-based authentication.
 */
public class GoogleAuthenticator implements Authenticator {
  private static final Logger LOG = LoggerFactory.getLogger(GoogleAuthenticator.class);

  /** The auth_provider name in the kubeconfig file. */
  static final String NAME = "google";

  /**
   * If a token is this close to expiry, consider it expired.
   *
   * @see com.google.auth.oauth2.OAuth2Credentials#DEFAULT_EXPIRATION_MARGIN
   */
  static final Duration DEFAULT_EXPIRATION_MARGIN = Duration.ofMinutes(5);

  /**
   * Config map key for the access token value.
   *
   * @see io.kubernetes.client.util.authenticators.GCPAuthenticator#ACCESS_TOKEN
   */
  static final String ACCESS_TOKEN = "access-token";

  /**
   * Config map key for the access token expiry.
   *
   * @see io.kubernetes.client.util.authenticators.GCPAuthenticator#EXPIRY
   */
  static final String EXPIRY = "expiry";

  /**
   * Config map key for the requested access token scopes.
   *
   * @see io.kubernetes.client.util.authenticators.GCPAuthenticator#SCOPES
   */
  static final String SCOPES = "scopes";

  /**
   * Default scopes to request for the access token, if none are configured in the kubeconfig file.
   *
   * @see io.kubernetes.client.util.authenticators.GCPAuthenticator#DEFAULT_SCOPES
   */
  static final String[] DEFAULT_SCOPES =
      new String[] {
        "https://www.googleapis.com/auth/cloud-platform",
        "https://www.googleapis.com/auth/userinfo.email"
      };

  public GoogleAuthenticator() {
    LOG.info("Registering Kubernetes auth_provider name=" + NAME);
  }

  @Override
  public String getName() {
    return NAME;
  }

  @Override
  @NotNull
  public String getToken(@Nullable Map<String, Object> config) {
    if (config == null || config.get(ACCESS_TOKEN) == null) {
      throw new AuthenticatorException("Config map does not contain access token");
    }
    return (String) config.get(ACCESS_TOKEN);
  }

  /**
   * Assumes that missing expiry date means isExpired=true.
   *
   * @see io.kubernetes.client.util.authenticators.GCPAuthenticator#isExpired(Map)
   */
  @Override
  public boolean isExpired(@Nullable Map<String, Object> config) {
    if (config == null) {
      LOG.warn("Assuming access token is expired, as config map is null");
      return true;
    }
    if (config.get(ACCESS_TOKEN) == null) {
      LOG.warn("Assuming access token is expired, as token is missing");
      return true;
    }
    Object expiryObj = config.get(EXPIRY);
    if (expiryObj == null) {
      LOG.warn("Assuming access token is expired, as expiry is missing");
      return true;
    }
    if (!(expiryObj instanceof Date)) {
      LOG.warn("Assuming access token is expired, as expiry is unexpected object type: " + expiryObj.getClass().getCanonicalName());
      return true;
    }
    var expiry = ((Date) expiryObj).toInstant();
    return expiry.compareTo(Instant.now().plus(DEFAULT_EXPIRATION_MARGIN)) <= 0;
  }

  /**
   * Refresh the token using the provided config.
   *
   * @see io.kubernetes.client.util.authenticators.GCPAuthenticator#refresh(Map)
   */
  @Override
  @NotNull
  public Map<String, Object> refresh(@Nullable Map<String, Object> config) {
    String[] scopes = parseScopes(config);
    try {
      var googleCredentials = GoogleCredentials.getApplicationDefault().createScoped(scopes);
      googleCredentials.refresh();
      var accessToken = googleCredentials.getAccessToken();
      var refreshed = new HashMap<String, Object>();
      refreshed.put(ACCESS_TOKEN, accessToken.getTokenValue());
      refreshed.put(EXPIRY, accessToken.getExpirationTime());
      refreshed.put(SCOPES, config != null ? config.get(SCOPES) : null);
      return refreshed;
    } catch (IOException e) {
      throw new AuthenticatorException("Could not create Application Default Credentials", e);
    }
  }

  /**
   * Parse the comma-separated list of scopes from the config map, or returns the default scopes if
   * none are provided.
   *
   * @see io.kubernetes.client.util.authenticators.GCPAuthenticator#parseScopes(Map)
   */
  @NotNull
  String[] parseScopes(@Nullable Map<String, Object> config) {
    String scopes = config != null ? (String) config.get(SCOPES) : null;
    if (scopes == null) {
      return DEFAULT_SCOPES;
    }
    if (Strings.isEmpty(scopes)) {
      return new String[] {};
    }
    return scopes.split(",");
  }
}
