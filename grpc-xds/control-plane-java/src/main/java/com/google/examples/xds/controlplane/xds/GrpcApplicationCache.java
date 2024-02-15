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
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

class GrpcApplicationCache {

  private final ConcurrentMap<ContextNamespace, Set<GrpcApplication>> cache =
      new ConcurrentHashMap<>();
  private final ReadWriteLock mux = new ReentrantReadWriteLock();

  /**
   * Replaces the contents of the gRPC application configuration cache with the provided new
   * application configuration.
   *
   * @param apps the new gRPC application configuration.
   * @return true if the application configuration cache changed as a result of this operation.
   */
  boolean set(
      @NotNull String kubecontext, @NotNull String namespace, @Nullable Set<GrpcApplication> apps) {
    if (apps == null) {
      apps = Collections.emptySet();
    }
    var key = key(kubecontext, namespace);
    Set<GrpcApplication> oldApps;
    try {
      mux.writeLock().lock();
      oldApps = cache.put(key, apps);
    } finally {
      mux.writeLock().unlock();
    }
    return !apps.equals(oldApps);
  }

  @NotNull
  Set<GrpcApplication> get(@NotNull String kubecontext, @NotNull String namespace) {
    try {
      mux.readLock().lock();
      return cache.get(key(kubecontext, namespace));
    } finally {
      mux.readLock().unlock();
    }
  }

  @NotNull
  Set<GrpcApplication> getAll() {
    var result = new HashSet<GrpcApplication>();
    try {
      mux.readLock().lock();
      for (Set<GrpcApplication> apps : cache.values()) {
        result.addAll(apps);
      }
    } finally {
      mux.readLock().unlock();
    }
    return result;
  }

  /**
   * GetOthers returns all cached gRPC applications _except_ the ones for the provided kubecontext
   * and namespace. Use this method when building a new xDS resource snapshot with existing apps and
   * updates from the Kubernetes informer for the provided kubecontext and namespace.
   */
  @NotNull
  Set<GrpcApplication> getOthers(@NotNull String kubecontext, @NotNull String namespace) {
    var keyToSkip = key(kubecontext, namespace);
    var result = new HashSet<GrpcApplication>();
    try {
      mux.readLock().lock();
      for (Map.Entry<ContextNamespace, Set<GrpcApplication>> entry : cache.entrySet()) {
        if (!keyToSkip.equals(entry.getKey())) {
          result.addAll(entry.getValue());
        }
      }
    } finally {
      mux.readLock().unlock();
    }
    return result;
  }

  @NotNull
  private ContextNamespace key(@NotNull String kubecontext, @NotNull String namespace) {
    return new ContextNamespace(kubecontext, namespace);
  }

  /** ContextNamespace is the key in the {@link GrpcApplicationCache}. */
  record ContextNamespace(@NotNull String kubecontext, @NotNull String namespace) {}
}
