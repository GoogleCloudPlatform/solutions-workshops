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

// Application represents an application, e.g., a gRPC server, that clients discover using xDS.
type Application struct {
	Namespace           string
	ServiceAccountName  string
	Name                string
	PathPrefix          string
	ServingPort         uint32
	ServingProtocol     string
	HealthCheckPort     uint32
	HealthCheckProtocol string
	Endpoints           []ApplicationEndpoints
}

// NewApplication is a convenience function that creates a Application where the
// k8s ServiceAccount and the application share the same name.
func NewApplication(namespace string, name string, servingPort uint32, servingProtocol string, healthCheckPort uint32, healthCheckProtocol string, endpoints []ApplicationEndpoints) Application {
	endpointsCopy := make([]ApplicationEndpoints, len(endpoints))
	copy(endpointsCopy, endpoints)
	slices.SortFunc(endpointsCopy, func(a ApplicationEndpoints, b ApplicationEndpoints) int {
		return a.Compare(b)
	})
	return Application{
		Namespace:           namespace,
		ServiceAccountName:  name,
		Name:                name,
		PathPrefix:          "",
		ServingPort:         servingPort,
		ServingProtocol:     servingProtocol,
		HealthCheckPort:     healthCheckPort,
		HealthCheckProtocol: healthCheckProtocol,
		Endpoints:           endpointsCopy,
	}
}

// Compare assumes that the list of endpoints is sorted,
// as done in `NewApplication()`.
func (a Application) Compare(b Application) int {
	if a.Namespace != b.Namespace {
		return strings.Compare(a.Namespace, b.Namespace)
	}
	if a.ServiceAccountName != b.ServiceAccountName {
		return strings.Compare(a.ServiceAccountName, b.ServiceAccountName)
	}
	if a.Name != b.Name {
		return strings.Compare(a.Name, b.Name)
	}
	if a.PathPrefix != b.PathPrefix {
		return strings.Compare(a.PathPrefix, b.PathPrefix)
	}
	if a.ServingPort != b.ServingPort {
		return int(a.ServingPort - b.ServingPort)
	}
	if a.ServingProtocol != b.ServingProtocol {
		return strings.Compare(a.ServingProtocol, b.ServingProtocol)
	}
	if a.HealthCheckPort != b.HealthCheckPort {
		return int(a.HealthCheckPort - b.HealthCheckPort)
	}
	if a.HealthCheckProtocol != b.HealthCheckProtocol {
		return strings.Compare(a.HealthCheckProtocol, b.HealthCheckProtocol)
	}
	return slices.CompareFunc(a.Endpoints, b.Endpoints,
		func(e ApplicationEndpoints, f ApplicationEndpoints) int {
			return e.Compare(f)
		})
}

// Equal assumes that the list of endpoints is sorted,
// as done in `NewApplication()`.
func (a Application) Equal(b Application) bool {
	return a.Compare(b) == 0
}
