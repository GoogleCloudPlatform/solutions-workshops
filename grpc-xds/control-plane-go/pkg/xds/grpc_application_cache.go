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

func (c *GRPCApplicationCache) Set(kubecontextName string, namespace string, apps []GRPCApplication) {
	if apps == nil {
		apps = []GRPCApplication{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key(kubecontextName, namespace)] = apps
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

// GetOthers returns all cached gRPC applications _except_ the ones for the provided kubecontext and namespace.
// Use this method when building a new xDS resource snapshot with existing apps and
// updates from the Kubernetes informer for the provided kubecontext and namespace.
func (c *GRPCApplicationCache) GetOthers(kubecontextName string, namespace string) []GRPCApplication {
	apps := []GRPCApplication{}
	keyToSkip := key(kubecontextName, namespace)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for key, appsForKey := range c.cache {
		if key != keyToSkip {
			apps = append(apps, appsForKey...)
		}
	}
	return apps
}

func key(kubecontextName string, namespace string) string {
	return fmt.Sprintf("%s/%s", kubecontextName, namespace)
}
