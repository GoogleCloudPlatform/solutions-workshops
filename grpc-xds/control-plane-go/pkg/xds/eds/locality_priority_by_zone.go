// Copyright 2024 Google LLC
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

package eds

import (
	"regexp"
)

// LocalityPriorityByZone determines EDS ClusterLoadAssignment locality priorites,
// based on the zone of the requesting node.
//
// Priorities start from 0 (highest), and must increase without any gaps. Multiple localities
// can share the same priority.
//
// Zone and region are names of cloud provider locations, e.g., the `us-central1-f`
// zone, and the `us-central1` region.
//
// Super-region is an invented term for this implementation. It is essentially the cloud provider
// region, excluding the numeric suffix. E.g., for the `us-west1` region, the
// super-region is `us-west`. This means that the `us-west1` and
// `us-west2` regions are in the same super-region. The intention is to provide a next level of
// priority, after considering zone and region. E.g., if traffic originates from `us-west1`,
// but no healthy endpoints are available in that region, we may prefer to send traffic to
// a nearby region, such as, e.g., `us-west2`, before considering other regions that are
// geographically more distant, e.g., `us-east1`.
//
// Multi-region is an invented term for this implementation. It is essentially the first part of
// the cloud provider region name, up to the first hyphen (`-`). E.g., for the region
// `us-east1`, the multi-region is `us`. This means that the `us-west1`
// and `us-east1` regions are in the same multi-region. The intention is to
// provide a next level of priority, after considering zone, region, and super-region. E.g., if
// traffic originates from `us-west1`, but no healthy endpoints are available in any of
// the `us-west1*` regions, we may prefer to send traffic to another region on the same
// continent, such as, e.g., `us-east1`, before considering regions on other continents,
// e.g., `europe-west1`.
type LocalityPriorityByZone struct{}

// BuildPriorityMap constructs the priority map for the provided zones, based on the zone of the requesting node.
// Assumption: The nodeHash value (the first argument) is the zone name of the requesting node.
func (l LocalityPriorityByZone) BuildPriorityMap(nodeZone string, zonesToPrioritize []string) map[string]uint32 {
	region := regionRegexp.FindString(nodeZone)
	superRegion := superRegionRegexp.FindString(nodeZone)
	multiRegion := multiRegionRegexp.FindString(nodeZone)
	zonesByLocalityMatch := map[LocalityMatch][]string{}
	for _, zoneToPrioritize := range zonesToPrioritize {
		switch {
		case nodeZone == zoneToPrioritize:
			zonesByLocalityMatch[Zone] = append(zonesByLocalityMatch[Zone], zoneToPrioritize)
		case region == regionRegexp.FindString(zoneToPrioritize):
			zonesByLocalityMatch[Region] = append(zonesByLocalityMatch[Region], zoneToPrioritize)
		case superRegion == superRegionRegexp.FindString(zoneToPrioritize):
			zonesByLocalityMatch[SuperRegion] = append(zonesByLocalityMatch[SuperRegion], zoneToPrioritize)
		case multiRegion == multiRegionRegexp.FindString(zoneToPrioritize):
			zonesByLocalityMatch[MultiRegion] = append(zonesByLocalityMatch[MultiRegion], zoneToPrioritize)
		default:
			zonesByLocalityMatch[Other] = append(zonesByLocalityMatch[Other], zoneToPrioritize)
		}
	}
	zonePriorities := map[string]uint32{}
	var priority uint32
	for match := Zone; match <= Other; match++ {
		zones := zonesByLocalityMatch[match]
		if len(zones) > 0 {
			for _, zone := range zones {
				zonePriorities[zone] = priority
			}
			priority++
		}
	}
	return zonePriorities
}

var _ LocalityPriorityMapper = &LocalityPriorityByZone{}

var (
	regionRegexp      = regexp.MustCompile("^[a-z]+-[a-z]+-?[0-9]+")
	superRegionRegexp = regexp.MustCompile("^[a-z]+-[a-z]+")
	multiRegionRegexp = regexp.MustCompile("^[a-z]+")
)

// LocalityMatch defines the priority order of matching or part-matching locality.
// In other words, the priority order is as follows:
//   - same zone
//   - same region
//   - same super-region
//   - same multi-region
//   - other
type LocalityMatch int

const (
	Zone LocalityMatch = iota
	Region
	SuperRegion
	MultiRegion
	Other
)
