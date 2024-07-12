// Copyright 2023 Google LLC
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

package com.google.examples.xds.controlplane.xds;

import java.util.Collection;
import java.util.Collections;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import org.jetbrains.annotations.NotNull;

class GrpcServerListenerCache<T> {

  private final ConcurrentMap<T, Set<EndpointAddress>> cache = new ConcurrentHashMap<>();

  /**
   * Adds the new server listener addresses to the cache.
   *
   * @return true if at least one of the addresses in newAddresses did not already exist in the
   *     cache for the nodeHash key.
   */
  boolean add(@NotNull T nodeHash, @NotNull Collection<EndpointAddress> newAddresses) {
    return cache.computeIfAbsent(nodeHash, g -> ConcurrentHashMap.newKeySet()).addAll(newAddresses);
  }

  @NotNull
  Set<EndpointAddress> get(@NotNull T nodeHash) {
    return Collections.unmodifiableSet(cache.getOrDefault(nodeHash, new HashSet<>(0)));
  }
}
