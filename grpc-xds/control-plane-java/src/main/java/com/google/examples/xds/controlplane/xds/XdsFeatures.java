// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.examples.xds.controlplane.xds;

import java.util.Map;
import java.util.Objects;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Contains flags to enable and disable xDS features. */
public record XdsFeatures(
    boolean serverListenerUsesRds,
    boolean enableDataPlaneTls,
    boolean requireDataPlaneClientCerts) {

  private static final Logger LOG = LoggerFactory.getLogger(XdsFeatures.class);

  /** Canonical constructor. */
  public XdsFeatures(
      boolean serverListenerUsesRds,
      boolean enableDataPlaneTls,
      boolean requireDataPlaneClientCerts) {
    if (!enableDataPlaneTls && requireDataPlaneClientCerts) {
      throw new IllegalArgumentException(
          "xDS feature flags: enableDataPlaneTls=true is required when"
              + " requireDataPlaneClientCerts=true");
    }
    if (serverListenerUsesRds) {
      LOG.warn(
          "gRPC-Go as of v1.60.1 does not support RouteConfiguration via RDS for server Listeners, see https://github.com/grpc/grpc-go/issues/6788");
    }
    this.serverListenerUsesRds = serverListenerUsesRds;
    this.enableDataPlaneTls = enableDataPlaneTls;
    this.requireDataPlaneClientCerts = requireDataPlaneClientCerts;
  }

  /**
   * Constructor used after parsing a YAML file (or similar) to a Map.
   *
   * <p>The map keys must match the instance variable names in this class.
   *
   * <p>Missing or null flags default to false.
   *
   * @param features xDS feature toggles
   */
  public XdsFeatures(@NotNull Map<String, Boolean> features) {
    this(
        Objects.requireNonNullElse(features.get("serverListenerUsesRds"), Boolean.FALSE)
            .booleanValue(),
        Objects.requireNonNullElse(features.get("enableDataPlaneTls"), Boolean.FALSE)
            .booleanValue(),
        Objects.requireNonNullElse(features.get("requireDataPlaneClientCerts"), Boolean.FALSE)
            .booleanValue());
  }
}
