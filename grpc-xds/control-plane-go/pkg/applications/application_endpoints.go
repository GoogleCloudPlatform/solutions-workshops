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

package applications

import (
	"slices"
	"strings"
)

type ApplicationEndpoints struct {
	Node           string
	Zone           string
	Addresses      []string
	EndpointStatus EndpointStatus
}

func NewApplicationEndpoints(node string, zone string, addresses []string, endpointStatus EndpointStatus) ApplicationEndpoints {
	addressesCopy := make([]string, len(addresses))
	copy(addressesCopy, addresses)
	slices.Sort(addressesCopy)
	return ApplicationEndpoints{
		Node:           node,
		Zone:           zone,
		Addresses:      addressesCopy,
		EndpointStatus: endpointStatus,
	}
}

// Compare assumes that the list of addresses is sorted,
// as done in `NewApplicationEndpoints()`.
func (e ApplicationEndpoints) Compare(f ApplicationEndpoints) int {
	if e.Node != f.Node {
		return strings.Compare(e.Node, f.Node)
	}
	if e.Zone != f.Zone {
		return strings.Compare(e.Zone, f.Zone)
	}
	if e.EndpointStatus != f.EndpointStatus {
		return strings.Compare(e.EndpointStatus.String(), f.EndpointStatus.String())
	}
	return slices.Compare(e.Addresses, f.Addresses)
}

// Equal assumes that the list of addresses is sorted,
// as done in `NewApplicationEndpoints()`.
func (e ApplicationEndpoints) Equal(f ApplicationEndpoints) bool {
	return e.Compare(f) == 0
}
