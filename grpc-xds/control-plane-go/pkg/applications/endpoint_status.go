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

package applications

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv1 "k8s.io/api/discovery/v1"
)

// EndpointStatus represents the serving status of an endpoint.
type EndpointStatus int

const (
	Healthy EndpointStatus = iota
	Unhealthy
	Draining
)

func EndpointStatusFromConditions(c discoveryv1.EndpointConditions) EndpointStatus {
	if c.Terminating != nil && *c.Terminating {
		return Draining
	}
	if c.Ready != nil && *c.Ready && c.Serving != nil && *c.Serving {
		// https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#ready
		return Healthy
	}
	return Unhealthy
}

func (e EndpointStatus) HealthStatus() corev3.HealthStatus {
	switch e {
	case Healthy:
		return corev3.HealthStatus_HEALTHY
	case Draining:
		return corev3.HealthStatus_DRAINING
	default:
		return corev3.HealthStatus_UNHEALTHY
	}
}

func (e EndpointStatus) String() string {
	switch e {
	case Healthy:
		return "Healthy"
	case Draining:
		return "Draining"
	default:
		return "Unhealthy"
	}
}
