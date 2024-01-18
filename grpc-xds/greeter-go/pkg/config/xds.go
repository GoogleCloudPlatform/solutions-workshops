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

package config

import (
	"context"
	"os"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/logging"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/xdsclient/bootstrap"
)

// UseXDS determines if the gRPC server should connect with an xDS control
// plane management server for configuration.
//
// See https://github.com/grpc/grpc-go/blob/v1.57.0/internal/envconfig/xds.go
func UseXDS() bool {
	if _, exists := os.LookupEnv("GRPC_XDS_BOOTSTRAP"); exists {
		return true
	}
	if _, exists := os.LookupEnv("GRPC_XDS_BOOTSTRAP_CONFIG"); exists {
		return true
	}
	return false
}

// UseXDSCredentials determines if the application should use xDS server- and
// client-side credentials, as described in
// [gRFC A29: xDS-Based Security for gRPC Clients and Servers]: https://github.com/grpc/proposal/blob/deaf1bcf248d1e48e83c470b00930cbd363fab6d/A29-xds-tls-security.md
//
// Returns trus if the gRPC xDS bootstrap configuration contains at least one
// instance in the `certificate_providers` section.
//
// See also https://github.com/grpc/grpc-go/blob/v1.57.0/xds/server.go#L224
func UseXDSCredentials(ctx context.Context) bool {
	bootstrapConfig, err := bootstrap.NewConfigPartial()
	if err != nil {
		logging.FromContext(ctx).Error(err, "Could not parse the gRPC xDS bootstrap configuration, using insecure credentials")
		return false
	}
	return len(bootstrapConfig.CertProviderConfigs) > 0
}
