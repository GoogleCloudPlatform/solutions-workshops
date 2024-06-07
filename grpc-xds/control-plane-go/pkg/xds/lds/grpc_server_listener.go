// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lds

import (
	"fmt"
	"net"
	"strconv"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
)

const (
	// GRPCServerListenerResourceNameTemplate uses the address and port above. Must match the template in the gRPC xDS bootstrap file, see
	// [gRFC A36: xDS-Enabled Servers]: https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol
	// Using the sample name from the gRPC-Go unit tests, but this is not important.
	GRPCServerListenerResourceNameTemplate = "grpc/server?xds.resource.listening_address=%s"
	// GRPCServerListenerRouteConfigurationName is used for the RouteConfiguration pointed to by server Listeners.
	GRPCServerListenerRouteConfigurationName = "default_inbound_config"
)

// CreateGRPCServerListener returns a downstream listener for xDS-enabled gRPC servers.
func CreateGRPCServerListener(host string, port uint32, enableTLS bool, requireClientCerts bool, enableRBAC bool) (*listenerv3.Listener, error) {
	statPrefix := GRPCServerListenerRouteConfigurationName
	httpConnectionManager, err := createDownstreamHTTPConnectionManager(GRPCServerListenerRouteConfigurationName, statPrefix, enableRBAC)
	if err != nil {
		return nil, fmt.Errorf("could not create HTTPConnectionManager for server LDS listener: %w", err)
	}

	// Relying on `serverListenerResourceNameTemplate` being an exact match of
	// `server_listener_resource_name_template` from the gRPC xDS bootstrap configuration. See
	// [gRFC A36: xDS-Enabled Servers]: https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol
	listenerName := fmt.Sprintf(GRPCServerListenerResourceNameTemplate, net.JoinHostPort(host, strconv.Itoa(int(port))))

	grpcServerListener, err := createServerListener(listenerName, host, port, httpConnectionManager, enableTLS, requireClientCerts)
	if err != nil {
		return nil, fmt.Errorf("could not create LDS Listener for gRPC servers: %w", err)
	}

	return grpcServerListener, nil
}
