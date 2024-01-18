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

import java.util.Collections;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

class GrpcApplicationCache {

  private final Set<GrpcApplication> cache = new HashSet<>();
  private final ReadWriteLock mux = new ReentrantReadWriteLock();

  /**
   * Replaces the contents of the gRPC application configuration cache with the provided new
   * application configuration.
   *
   * @param apps the new gRPC application configuration.
   * @return true if the cache contents changed, false otherwise.
   */
  boolean set(@Nullable Set<GrpcApplication> apps) {
    if (apps == null) {
      apps = Collections.emptySet();
    }
    try {
      mux.writeLock().lock();
      if (cache.equals(apps)) {
        return false;
      }
      cache.clear();
      cache.addAll(apps);
      return true;
    } finally {
      mux.writeLock().unlock();
    }
  }

  @NotNull
  Set<GrpcApplication> get() {
    try {
      mux.readLock().lock();
      return Collections.unmodifiableSet(cache);
    } finally {
      mux.readLock().unlock();
    }
  }
}
