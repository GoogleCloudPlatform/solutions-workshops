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

package com.google.examples.xds.controlplane.xds;

import static org.junit.jupiter.api.Assertions.*;

import io.envoyproxy.envoy.config.core.v3.Locality;
import java.util.List;
import java.util.Map;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;
import org.junit.jupiter.api.Test;

/** Test and document the regular expressions. */
public class LocalityPrioritiesByZoneTest {

  @NotNull
  private static Locality zone(@Nullable String zone) {
    var localityBuilder = Locality.newBuilder();
    if (zone != null) {
      localityBuilder.setZone(zone);
    }
    return localityBuilder.build();
  }

  @Test
  void buildPriorityMap() {
    Map<Locality, Integer> priorityMap =
        new LocalityPriorityByZone()
            .buildPriorityMap(
                "us-west1-a",
                List.of(
                    zone("us-west1-a"),
                    zone("us-west1-b"),
                    zone("us-west2-a"),
                    zone("us-west2-b"),
                    zone("us-east1-b"),
                    zone("us-central1-f"),
                    zone("europe-north12-a"),
                    zone("australia-southeast1-a")));
    assertEquals(priorityMap.size(), 8);
    assertEquals(priorityMap.get(zone("us-west1-a")), 0);
    assertEquals(priorityMap.get(zone("us-west1-b")), 1);
    assertEquals(priorityMap.get(zone("us-west2-a")), 2);
    assertEquals(priorityMap.get(zone("us-west2-b")), 2);
    assertEquals(priorityMap.get(zone("us-east1-b")), 3);
    assertEquals(priorityMap.get(zone("us-central1-f")), 3);
    assertEquals(priorityMap.get(zone("europe-north12-a")), 4);
    assertEquals(priorityMap.get(zone("australia-southeast1-a")), 4);
  }

  @Test
  void sameZone() {
    assertTrue(LocalityPriorityByZone.sameZone("us-central1-f", zone("us-central1-f")));
    assertTrue(
        LocalityPriorityByZone.sameZone("australia-southeast1-a", zone("australia-southeast1-a")));
    assertTrue(LocalityPriorityByZone.sameZone("europe-west12-c", zone("europe-west12-c")));
    assertFalse(LocalityPriorityByZone.sameZone("us-central1-a", zone("us-central1-f")));
    assertFalse(
        LocalityPriorityByZone.sameZone("australia-southeast1-a", zone("australia-southeast1-b")));
    assertFalse(LocalityPriorityByZone.sameZone("europe-west12-c", zone("europe-west12-a")));
    assertFalse(LocalityPriorityByZone.sameZone("us-west1-a", zone("europe-west1-a")));
    assertFalse(LocalityPriorityByZone.sameZone("us-west1-a", zone(null)));
  }

  @Test
  void sameRegion() {
    assertTrue(LocalityPriorityByZone.sameRegion("us-central1", zone("us-central1-f")));
    assertTrue(
        LocalityPriorityByZone.sameRegion("australia-southeast1", zone("australia-southeast1-a")));
    assertTrue(LocalityPriorityByZone.sameRegion("europe-west12", zone("europe-west12-c")));
    assertFalse(LocalityPriorityByZone.sameRegion("us-central1", zone("us-west1-a")));
    assertFalse(
        LocalityPriorityByZone.sameRegion("australia-southeast1", zone("australia-southeast2-a")));
    assertFalse(LocalityPriorityByZone.sameRegion("europe-west10", zone("europe-west12-a")));
    assertFalse(LocalityPriorityByZone.sameRegion("us-west1", zone(null)));
  }

  @Test
  void sameSuperRegion() {
    assertTrue(LocalityPriorityByZone.sameSuperRegion("us-central", zone("us-central1-f")));
    assertTrue(
        LocalityPriorityByZone.sameSuperRegion(
            "australia-southeast", zone("australia-southeast1-a")));
    assertTrue(LocalityPriorityByZone.sameSuperRegion("europe-west", zone("europe-west12-c")));
    assertFalse(LocalityPriorityByZone.sameSuperRegion("us-central", zone("us-west1-a")));
    assertFalse(LocalityPriorityByZone.sameSuperRegion("europe-north", zone("europe-west12-a")));
    assertFalse(LocalityPriorityByZone.sameSuperRegion("us-west", zone(null)));
  }

  @Test
  void sameMultiRegion() {
    assertTrue(LocalityPriorityByZone.sameMultiRegion("us", zone("us-central1-f")));
    assertTrue(LocalityPriorityByZone.sameMultiRegion("australia", zone("australia-southeast1-a")));
    assertTrue(LocalityPriorityByZone.sameMultiRegion("europe", zone("europe-west12-c")));
    assertFalse(LocalityPriorityByZone.sameMultiRegion("europe", zone("us-west1-a")));
    assertFalse(LocalityPriorityByZone.sameMultiRegion("australia", zone("us-west1-a")));
    assertFalse(LocalityPriorityByZone.sameMultiRegion("us", zone("europe-west12-a")));
    assertFalse(LocalityPriorityByZone.sameMultiRegion("us", zone(null)));
  }
}
