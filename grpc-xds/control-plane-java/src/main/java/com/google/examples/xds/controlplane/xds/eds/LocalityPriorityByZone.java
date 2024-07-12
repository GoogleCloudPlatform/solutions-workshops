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
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

/**
 * Determines EDS ClusterLoadAssignment locality priorites, based on the zone of the requesting
 * node.
 *
 * <p>Priorities start from 0 (highest), and must increase without any gaps. Multiple localities can
 * share the same priority.
 *
 * <p>Zone and region are names of cloud provider locations, e.g., the <code>us-central1-f</code>
 * zone, and the <code>us-central1</code> region.
 *
 * <p>Super-region is an invented term for this implementation. It is essentially the cloud provider
 * region, excluding the numeric suffix. E.g., for the <code>us-west1</code> region, the
 * super-region is <code>us-west</code>. This means that the <code>us-west1</code> and <code>
 * us-west2</code> regions are in the same super-region. The intention is to provide a next level of
 * priority, after considering zone and region. E.g., if traffic originates from <code>us-west1
 * </code>, but no healthy endpoints are available in that region, we may prefer to send traffic to
 * a nearby region, such as, e.g., <code>us-west2</code>, before considering other regions that are
 * geographically more distant, e.g., <code>us-east1</code>.
 *
 * <p>Multi-region is an invented term for this implementation. It is essentially the first part of
 * the cloud provider region name, up to the first hyphen (<code>-</code>). E.g., for the region
 * <code>us-east1</code>, the multi-region is <code>us</code>. This means that the <code>us-west1
 * </code> and <code>us-east1</code> regions are in the same multi-region. The intention is to
 * provide a next level of priority, after considering zone, region, and super-region. E.g., if
 * traffic originates from <code>us-west1</code>, but no healthy endpoints are available in any of
 * the <code>us-west1*</code> regions, we may prefer to send traffic to another region on the same
 * continent, such as, e.g., <code>us-east1</code>, before considering regions on other continents,
 * e.g., <code>europe-west1</code>.
 */
public class LocalityPriorityByZone implements LocalityPriorityMapper<String> {
  private static final Pattern REGION_PATTERN = Pattern.compile("^[a-z]+-[a-z]+-?[0-9]+");
  private static final Pattern SUPER_REGION_PATTERN = Pattern.compile("^[a-z]+-[a-z]+");
  private static final Pattern MULTI_REGION_PATTERN = Pattern.compile("^[a-z]+");

  /**
   * Constructs the priority map for the provided localities, based on the zone of the requesting
   * node.
   *
   * <p>Only the zone field of the localities is used to determine the priority, not the region or
   * sub-zone.
   *
   * @param zone of the requesting node
   * @param localities localities to prioritize.
   */
  @Override
  @NotNull
  public Map<Locality, Integer> buildPriorityMap(
      @NotNull String zone, @NotNull Collection<Locality> localities) {
    Matcher regionMatcher = REGION_PATTERN.matcher(zone);
    String region = regionMatcher.find() ? regionMatcher.group() : null;
    Matcher superRegionMatcher = SUPER_REGION_PATTERN.matcher(zone);
    String superRegion = superRegionMatcher.find() ? superRegionMatcher.group() : null;
    Matcher multiRegionMatcher = MULTI_REGION_PATTERN.matcher(zone);
    String multiRegion = multiRegionMatcher.find() ? multiRegionMatcher.group() : null;
    Map<LocalityMatch, Set<Locality>> localitiesByMatch = new HashMap<>();
    for (Locality locality : localities) {
      if (sameZone(zone, locality)) {
        localitiesByMatch.computeIfAbsent(LocalityMatch.ZONE, l -> new HashSet<>()).add(locality);
      } else if (sameRegion(region, locality)) {
        localitiesByMatch.computeIfAbsent(LocalityMatch.REGION, l -> new HashSet<>()).add(locality);
      } else if (sameSuperRegion(superRegion, locality)) {
        localitiesByMatch
            .computeIfAbsent(LocalityMatch.SUPER_REGION, l -> new HashSet<>())
            .add(locality);
      } else if (sameMultiRegion(multiRegion, locality)) {
        localitiesByMatch
            .computeIfAbsent(LocalityMatch.MULTI_REGION, l -> new HashSet<>())
            .add(locality);
      } else {
        localitiesByMatch.computeIfAbsent(LocalityMatch.OTHER, l -> new HashSet<>()).add(locality);
      }
    }
    Map<Locality, Integer> priorityMap = new HashMap<>();
    int priority = 0;
    for (LocalityMatch localityMatch : LocalityMatch.values()) {
      Set<Locality> localitiesForMatch = localitiesByMatch.get(localityMatch);
      if (localitiesForMatch != null && !localitiesForMatch.isEmpty()) {
        for (Locality locality : localitiesForMatch) {
          priorityMap.put(locality, priority);
        }
        priority++;
      }
    }
    return priorityMap;
  }

  static boolean sameZone(@Nullable String zone, @NotNull Locality locality) {
    return locality.getZone().equals(zone);
  }

  static boolean sameRegion(@Nullable String region, @NotNull Locality locality) {
    Matcher matcher = REGION_PATTERN.matcher(locality.getZone());
    return matcher.find() && matcher.group().equals(region);
  }

  static boolean sameSuperRegion(@Nullable String superRegion, @NotNull Locality locality) {
    Matcher matcher = SUPER_REGION_PATTERN.matcher(locality.getZone());
    return matcher.find() && matcher.group().equals(superRegion);
  }

  static boolean sameMultiRegion(@Nullable String multiRegion, @NotNull Locality locality) {
    Matcher matcher = MULTI_REGION_PATTERN.matcher(locality.getZone());
    return matcher.find() && matcher.group().equals(multiRegion);
  }

  /**
   * LocalityMatch defines the priority order of matching or part-matching locality.
   *
   * <p>In other words, the priority order is as follows:
   *
   * <ol>
   *   <li>same zone
   *   <li>same region
   *   <li>same super-region
   *   <li>same multi-region
   *   <li>other
   * </ol>
   */
  private enum LocalityMatch {
    ZONE,
    REGION,
    SUPER_REGION,
    MULTI_REGION,
    OTHER
  }
}
