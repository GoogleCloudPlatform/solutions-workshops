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
	Name       string
	PathPrefix string
	Port       uint32
	Endpoints  []GRPCApplicationEndpoints
}

func NewGRPCApplication(name string, port uint32, endpoints []GRPCApplicationEndpoints) GRPCApplication {
	if endpoints == nil {
		endpoints = []GRPCApplicationEndpoints{}
	}
	return GRPCApplication{
		Name:       name,
		PathPrefix: "",
		Port:       port,
		Endpoints:  endpoints,
	}
}

func (a *GRPCApplication) listenerName() string {
	return a.Name
}

func (a *GRPCApplication) routeConfigName() string {
	return a.Name
}

func (a *GRPCApplication) clusterName() string {
	return a.Name
}
