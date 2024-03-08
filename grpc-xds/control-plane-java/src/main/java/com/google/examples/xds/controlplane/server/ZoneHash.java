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

package com.google.examples.xds.controlplane.server;

import io.envoyproxy.controlplane.cache.NodeGroup;
import io.envoyproxy.envoy.config.core.v3.Node;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

/** Use locality.zone as the node hash value. */
public class ZoneHash implements NodeGroup<String> {
  public static final String DEFAULT_NODE_HASH = "";

  @Override
  @NotNull
  public String hash(@Nullable Node node) {
    if (node == null) {
      return DEFAULT_NODE_HASH;
    }
    return node.getLocality().getZone();
  }
}
