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

type GRPCApplication struct {
	namespace              string
	serviceAccountName     string
	listenerName           string
	routeConfigurationName string
	clusterName            string
	pathPrefix             string
	port                   uint32
	endpoints              []GRPCApplicationEndpoints
}

// NewGRPCApplication is a convenience function that creates a GRPCApplication where the
// k8s ServiceAccount, LDS Listener, RDS RouteConfiguration, and CDS Cluster all share the same name.
func NewGRPCApplication(namespace string, name string, port uint32, endpoints []GRPCApplicationEndpoints) GRPCApplication {
	endpointsCopy := make([]GRPCApplicationEndpoints, len(endpoints))
	copy(endpointsCopy, endpoints)
	return GRPCApplication{
		namespace:              namespace,
		serviceAccountName:     name,
		listenerName:           name,
		routeConfigurationName: name,
		clusterName:            name,
		pathPrefix:             "",
		port:                   port,
		endpoints:              endpointsCopy,
	}
}

func (a *GRPCApplication) Namespace() string {
	return a.namespace
}

func (a *GRPCApplication) ServiceAccountName() string {
	return a.serviceAccountName
}

func (a *GRPCApplication) ListenerName() string {
	return a.listenerName
}

func (a *GRPCApplication) RouteConfigurationName() string {
	return a.routeConfigurationName
}

func (a *GRPCApplication) ClusterName() string {
	return a.clusterName
}

func (a *GRPCApplication) PathPrefix() string {
	return a.pathPrefix
}

func (a *GRPCApplication) Port() uint32 {
	return a.port
}

func (a *GRPCApplication) Endpoints() []GRPCApplicationEndpoints {
	endpointsCopy := make([]GRPCApplicationEndpoints, len(a.endpoints))
	copy(endpointsCopy, a.endpoints)
	return endpointsCopy
}
