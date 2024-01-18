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

type GRPCApplicationCache struct {
	mu    sync.RWMutex
	cache []GRPCApplication
}

func NewGRPCApplicationCache() *GRPCApplicationCache {
	return &GRPCApplicationCache{
		cache: []GRPCApplication{},
	}
}

func (c *GRPCApplicationCache) Set(apps []GRPCApplication) {
	if apps == nil {
		apps = []GRPCApplication{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = apps
}

func (c *GRPCApplicationCache) Get() []GRPCApplication {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache
}
