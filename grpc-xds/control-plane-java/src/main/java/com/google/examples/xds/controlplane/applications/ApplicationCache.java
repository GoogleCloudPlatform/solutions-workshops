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

package com.google.examples.xds.controlplane.applications;

import java.util.Collections;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

/**
 * Stores information about applications discovered from EndpointSlices, by kubecontext and
 * namespace.
 */
public class ApplicationCache {

  private final ConcurrentMap<ContextNamespace, Set<Application>> cache = new ConcurrentHashMap<>();
  private final ReadWriteLock mux = new ReentrantReadWriteLock();

  /**
   * Replaces the contents of the application configuration cache with the provided new application
   * configuration.
   *
   * @param apps the new application configuration.
   * @return true if the application configuration cache changed as a result of this operation.
   */
  public boolean set(
      @NotNull String kubecontext, @NotNull String namespace, @Nullable Set<Application> apps) {
    if (apps == null) {
      apps = Collections.emptySet();
    }
    var key = key(kubecontext, namespace);
    Set<Application> oldApps;
    mux.writeLock().lock();
    try {
      oldApps = cache.put(key, apps);
    } finally {
      mux.writeLock().unlock();
    }
    return !apps.equals(oldApps);
  }

  /** Get the known applications for the provided kubecontext and namespace. */
  @SuppressWarnings("unused")
  @NotNull
  public Set<Application> get(@NotNull String kubecontext, @NotNull String namespace) {
    mux.readLock().lock();
    try {
      return cache.get(key(kubecontext, namespace));
    } finally {
      mux.readLock().unlock();
    }
  }

  /** Get all known applications. */
  @NotNull
  public Set<Application> getAll() {
    var result = new HashSet<Application>();
    mux.readLock().lock();
    try {
      for (Set<Application> apps : cache.values()) {
        result.addAll(apps);
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

  /** ContextNamespace is the key in the {@link ApplicationCache}. */
  record ContextNamespace(@NotNull String kubecontext, @NotNull String namespace) {}
}
