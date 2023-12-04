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

import (
	"sync"
)

type ServerListenerCache struct {
	mu    sync.RWMutex
	cache map[string]map[EndpointAddress]bool
}

func NewServerListenerCache() *ServerListenerCache {
	return &ServerListenerCache{
		cache: map[string]map[EndpointAddress]bool{},
	}
}

// Add returns true if at least one of the new server listener addresses did not already exist in the cache
// for the provided `nodeHash` cache key.
func (c *ServerListenerCache) Add(nodeHash string, newAddresses []EndpointAddress) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	addresses, exists := c.cache[nodeHash]
	if !exists {
		addresses = make(map[EndpointAddress]bool, len(newAddresses))
		c.cache[nodeHash] = addresses
	}
	added := false
	for _, address := range newAddresses {
		if !addresses[address] {
			addresses[address] = true
			added = true
		}
	}
	return added
}

func (c *ServerListenerCache) Get(nodeHash string) []EndpointAddress {
	c.mu.RLock()
	defer c.mu.RUnlock()
	addresses := make([]EndpointAddress, len(c.cache[nodeHash]))
	i := 0
	for address := range c.cache[nodeHash] {
		addresses[i] = address
		i++
	}
	return addresses
}
