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
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

const (
	// Fixed node hash, so all xDS clients access the same cache snapshot.
	id = "default"
)

// FixedHash uses a fixed value as the node hash.
type FixedHash struct{}

var _ cachev3.NodeHash = &FixedHash{}

func (FixedHash) ID(_ *corev3.Node) string {
	return id
}
