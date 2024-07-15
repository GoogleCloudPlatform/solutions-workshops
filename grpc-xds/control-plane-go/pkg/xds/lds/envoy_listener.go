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

package lds

import (
	"fmt"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
)

const (
	envoyGRPCListenerNamePrefix             = "envoy-listener"
	EnvoyGRPCListenerRouteConfigurationName = "envoy-route-configuration"
	envoyListenerSocketAddress              = "0.0.0.0"
)

// CreateEnvoyGRPCListener returns a GRPC listener for Envoy front proxies.
func CreateEnvoyGRPCListener(port uint32, enableTLS bool) (*listenerv3.Listener, error) {
	listenerName := fmt.Sprintf("%s-%d", envoyGRPCListenerNamePrefix, port)
	httpConnectionManager, err := createHTTPConnectionManagerForSocketListener(EnvoyGRPCListenerRouteConfigurationName, listenerName, false)
	if err != nil {
		return nil, fmt.Errorf("could not create HttpConnectionManager for Envoy gRPC LDS Listener: %w", err)
	}
	envoyGRPCListener, err := createSocketListener(listenerName, envoyListenerSocketAddress, port, httpConnectionManager, enableTLS, false)
	if err != nil {
		return nil, fmt.Errorf("could not create LDS Listener for Envoy proxy: %w", err)
	}
	return envoyGRPCListener, nil
}
