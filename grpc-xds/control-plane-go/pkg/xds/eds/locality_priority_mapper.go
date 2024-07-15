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

// LocalityPriorityMapper determines EDS ClusterLoadAssignment locality priorites.
type LocalityPriorityMapper interface {
	// BuildPriorityMap constructs the priority map for the provided zones, based on the node hash.
	BuildPriorityMap(nodeHash string, zones []string) map[string]uint32
}

// FixedLocalityPriority returns an empty map.
// Lookups in the map will always return 0 as the value, so all localities can be assigned the highest priority.
type FixedLocalityPriority struct{}

func (f FixedLocalityPriority) BuildPriorityMap(_ string, _ []string) map[string]uint32 {
	return make(map[string]uint32)
}

var _ LocalityPriorityMapper = &FixedLocalityPriority{}
