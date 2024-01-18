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

package xds

type GRPCApplicationEndpoints struct {
	node      string
	zone      string
	addresses []string
}

func NewGRPCApplicationEndpoints(node string, zone string, addresses []string) GRPCApplicationEndpoints {
	addressesCopy := make([]string, len(addresses))
	copy(addressesCopy, addresses)
	return GRPCApplicationEndpoints{
		node:      node,
		zone:      zone,
		addresses: addressesCopy,
	}
}

func (e *GRPCApplicationEndpoints) Node() string {
	return e.node
}

func (e *GRPCApplicationEndpoints) Zone() string {
	return e.zone
}

func (e *GRPCApplicationEndpoints) Addresses() []string {
	addressesCopy := make([]string, len(e.addresses))
	copy(addressesCopy, e.addresses)
	return addressesCopy
}
