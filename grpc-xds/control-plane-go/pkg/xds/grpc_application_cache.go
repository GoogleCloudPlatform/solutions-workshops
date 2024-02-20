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
	"fmt"
	"slices"
	"sync"
)

// The GRPCApplicationCache key is `<kubecontext>/<namespace>`.
type GRPCApplicationCache struct {
	mu    sync.RWMutex
	cache map[string][]GRPCApplication
}

func NewGRPCApplicationCache() *GRPCApplicationCache {
	return &GRPCApplicationCache{
		cache: map[string][]GRPCApplication{},
	}
}

// Put returns true iff the update changed the cache.
// Use the return value to avoid sending xDS updates to clients when nothing has changed.
func (c *GRPCApplicationCache) Put(kubecontextName string, namespace string, apps []GRPCApplication) bool {
	if apps == nil {
		apps = []GRPCApplication{}
	}
	slices.SortFunc(apps, func(a GRPCApplication, b GRPCApplication) int {
		return a.Compare(b)
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	key := key(kubecontextName, namespace)
	oldApps := c.cache[key]
	c.cache[key] = apps
	return !slices.EqualFunc(oldApps, apps, func(a GRPCApplication, b GRPCApplication) bool {
		return a.Equal(b)
	})
}

func (c *GRPCApplicationCache) Get(kubecontextName string, namespace string) []GRPCApplication {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache[key(kubecontextName, namespace)]
}

func (c *GRPCApplicationCache) GetAll() []GRPCApplication {
	apps := []GRPCApplication{}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, appsForKey := range c.cache {
		apps = append(apps, appsForKey...)
	}
	return apps
}

func key(kubecontextName string, namespace string) string {
	return fmt.Sprintf("%s/%s", kubecontextName, namespace)
}
