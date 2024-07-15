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

/** Contains flags to enable and disable xDS features. */
public record XdsFeatures(
    boolean enableControlPlaneTls,
    boolean requireControlPlaneClientCerts,
    boolean enableDataPlaneTls,
    boolean requireDataPlaneClientCerts,
    boolean enableRbac,
    boolean enableFederation) {

  /** Canonical constructor. */
  public XdsFeatures {
    if (requireControlPlaneClientCerts && !enableControlPlaneTls) {
      throw new IllegalArgumentException(
          "requireControlPlaneClientCerts=true requires enableControlPlaneTls=true");
    }
    if (requireDataPlaneClientCerts && !enableDataPlaneTls) {
      throw new IllegalArgumentException(
          "requireDataPlaneClientCerts=true requires enableDataPlaneTls=true");
    }
    if (enableRbac && (!enableDataPlaneTls || !requireDataPlaneClientCerts)) {
      throw new IllegalArgumentException(
          "enableRbac=true requires enableDataPlaneTls=true and requireDataPlaneClientCerts=true");
    }
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
        Objects.requireNonNullElse(features.get("enableControlPlaneTls"), Boolean.FALSE),
        Objects.requireNonNullElse(features.get("requireControlPlaneClientCerts"), Boolean.FALSE),
        Objects.requireNonNullElse(features.get("enableDataPlaneTls"), Boolean.FALSE),
        Objects.requireNonNullElse(features.get("requireDataPlaneClientCerts"), Boolean.FALSE),
        Objects.requireNonNullElse(features.get("enableRbac"), Boolean.FALSE),
        Objects.requireNonNullElse(features.get("enableFederation"), Boolean.FALSE));
  }
}
