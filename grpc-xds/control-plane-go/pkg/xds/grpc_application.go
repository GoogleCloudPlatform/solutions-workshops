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
	"slices"
	"strings"
)

type GRPCApplication struct {
	Namespace              string
	ServiceAccountName     string
	ListenerName           string
	RouteConfigurationName string
	ClusterName            string
	EDSServiceName         string
	PathPrefix             string
	Port                   uint32
	Endpoints              []GRPCApplicationEndpoints
}

// NewGRPCApplication is a convenience function that creates a GRPCApplication where the
// k8s ServiceAccount, LDS Listener, RDS RouteConfiguration, CDS Cluster, and EDS ServiceName all share the same name.
func NewGRPCApplication(namespace string, name string, port uint32, endpoints []GRPCApplicationEndpoints) GRPCApplication {
	endpointsCopy := make([]GRPCApplicationEndpoints, len(endpoints))
	copy(endpointsCopy, endpoints)
	slices.SortFunc(endpointsCopy, func(a GRPCApplicationEndpoints, b GRPCApplicationEndpoints) int {
		return a.Compare(b)
	})
	return GRPCApplication{
		Namespace:              namespace,
		ServiceAccountName:     name,
		ListenerName:           name,
		RouteConfigurationName: name,
		ClusterName:            name,
		EDSServiceName:         name,
		PathPrefix:             "",
		Port:                   port,
		Endpoints:              endpointsCopy,
	}
}

// Compare assumes that the list of endpoints is sorted,
// as done in `NewGRPCApplication()`.
func (a GRPCApplication) Compare(b GRPCApplication) int {
	if a.Namespace != b.Namespace {
		return strings.Compare(a.Namespace, b.Namespace)
	}
	if a.ServiceAccountName != b.ServiceAccountName {
		return strings.Compare(a.ServiceAccountName, b.ServiceAccountName)
	}
	if a.ListenerName != b.ListenerName {
		return strings.Compare(a.ListenerName, b.ListenerName)
	}
	if a.RouteConfigurationName != b.RouteConfigurationName {
		return strings.Compare(a.RouteConfigurationName, b.RouteConfigurationName)
	}
	if a.ClusterName != b.ClusterName {
		return strings.Compare(a.ClusterName, b.ClusterName)
	}
	if a.EDSServiceName != b.EDSServiceName {
		return strings.Compare(a.EDSServiceName, b.EDSServiceName)
	}
	if a.PathPrefix != b.PathPrefix {
		return strings.Compare(a.PathPrefix, b.PathPrefix)
	}
	if a.Port != b.Port {
		return int(a.Port - b.Port)
	}
	return slices.CompareFunc(a.Endpoints, b.Endpoints,
		func(e GRPCApplicationEndpoints, f GRPCApplicationEndpoints) int {
			return e.Compare(f)
		})
}

// Equal assumes that the list of endpoints is sorted,
// as done in `NewGRPCApplication()`.
func (a GRPCApplication) Equal(b GRPCApplication) bool {
	return a.Compare(b) == 0
}
