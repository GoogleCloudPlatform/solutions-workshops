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

package com.google.examples.xds.controlplane.xds.eds;

import io.envoyproxy.envoy.config.core.v3.Locality;
import java.util.Collection;
import java.util.Map;
import org.jetbrains.annotations.NotNull;

/** Determines EDS ClusterLoadAssignment locality priorites. */
public interface LocalityPriorityMapper<T> {

  /**
   * Constructs the priority map for the provided localities, based on information in the node hash.
   *
   * @param nodeHash hash of the node information.
   * @param localities localities to prioritize.
   */
  @NotNull
  Map<Locality, Integer> buildPriorityMap(
      @NotNull T nodeHash, @NotNull Collection<Locality> localities);
}
