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

import io.envoyproxy.controlplane.cache.ConfigWatcher;
import io.envoyproxy.controlplane.cache.DeltaResponse;
import io.envoyproxy.controlplane.cache.DeltaWatch;
import io.envoyproxy.controlplane.cache.DeltaXdsRequest;
import io.envoyproxy.controlplane.cache.NodeGroup;
import io.envoyproxy.controlplane.cache.Resources.ResourceType;
import io.envoyproxy.controlplane.cache.Response;
import io.envoyproxy.controlplane.cache.Watch;
import io.envoyproxy.controlplane.cache.XdsRequest;
import io.envoyproxy.controlplane.cache.v3.SimpleCache;
import io.envoyproxy.controlplane.cache.v3.Snapshot;
import java.util.Map;
import java.util.Set;
import java.util.function.Consumer;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * XdsSnapshotCache stores snapshots of xDS resources in a delegate cache.
 *
 * <p>It handles the initial server listener and route configuration request by intercepting
 * Listener stream creation, see createWatch().
 *
 * <p>It also handles propagating snapshots to all node groups in the cache. This is currently done
 * naively, by using the same snapshot for all node groups.
 *
 * @param <T> node group type
 */
public class XdsSnapshotCache<T> implements ConfigWatcher {
  private static final Logger LOG = LoggerFactory.getLogger(XdsSnapshotCache.class);
  private final SimpleCache<T> delegate;
  private final NodeGroup<T> nodeGroup;

  /**
   * Create an xDS resource cache for the provided node group mapping.
   *
   * @param nodeGroup function that returns a consistent identifier for a node.
   */
  public XdsSnapshotCache(@NotNull NodeGroup<T> nodeGroup) {
    this.delegate = new SimpleCache<>(nodeGroup);
    this.nodeGroup = nodeGroup;
  }

  /** Naively sets the same snapshot for all node groups in the cache. */
  public void setSnapshot(@NotNull Snapshot snapshot) {
    delegate.groups().forEach(nodeGroup -> delegate.setSnapshot(nodeGroup, snapshot));
  }

  /**
   * CreateWatch intercepts stream creation, and if it is a new Listener stream, creates a snapshot
   * with the server listener and associated route configuration.
   *
   * <p>This solves (in a slightly hacky way) bootstrapping of xDS-enabled gRPC servers.
   */
  @Override
  public Watch createWatch(
      boolean ads,
      XdsRequest request,
      Set<String> knownResourceNames,
      Consumer<Response> responseConsumer,
      boolean hasClusterChanged,
      boolean allowDefaultEmptyEdsUpdate) {
    // TODO: Should also look for the server listener in the resource names of the request.
    if (request != null && request.getResourceType() == ResourceType.LISTENER) {
      T group = nodeGroup.hash(request.v3Request().getNode());
      if (delegate.getSnapshot(group) == null) {
        LOG.info(
            "Missing snapshot, creating a snapshot with server listener and"
                + " route configuration for nodeGroup={}",
            group);
        delegate.setSnapshot(group, new XdsSnapshotBuilder().build());
      }
    }
    return delegate.createWatch(
        ads,
        request,
        knownResourceNames,
        responseConsumer,
        hasClusterChanged,
        allowDefaultEmptyEdsUpdate);
  }

  /** Just delegating, since delta xDS is not supported by this control plane implementation. */
  @Override
  public DeltaWatch createDeltaWatch(
      DeltaXdsRequest request,
      String requesterVersion,
      Map<String, String> resourceVersions,
      Set<String> pendingResources,
      boolean isWildcard,
      Consumer<DeltaResponse> responseConsumer,
      boolean hasClusterChanged) {
    return delegate.createDeltaWatch(
        request,
        requesterVersion,
        resourceVersions,
        pendingResources,
        isWildcard,
        responseConsumer,
        hasClusterChanged);
  }
}
