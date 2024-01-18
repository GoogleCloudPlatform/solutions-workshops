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

// Modified from the original
// [source]: https://github.com/grpc/grpc-go/blob/v1.57.0/xds/internal/xdsclient/bootstrap/bootstrap.go
// by only parsing a subset of the gRPC xDS bootstrap configuration.
// Copyright 2019 gRPC authors
// Licensed under the Apache License, Version 2.0

// Package bootstrap provides the functionality to initialize certain aspects
// of an xDS client by reading a bootstrap file.
package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"

	v3corepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"google.golang.org/grpc/credentials/tls/certprovider"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	// XDSBootstrapFileNameEnv is the env variable to set bootstrap file name.
	// Do not use this and read from env directly. Its value is read and kept in
	// variable XDSBootstrapFileName.
	//
	// When both bootstrap FileName and FileContent are set, FileName is used.
	XDSBootstrapFileNameEnv = "GRPC_XDS_BOOTSTRAP"
	// XDSBootstrapFileContentEnv is the env variable to set bootstrap file
	// content. Do not use this and read from env directly. Its value is read
	// and kept in variable XDSBootstrapFileContent.
	//
	// When both bootstrap FileName and FileContent are set, FileName is used.
	XDSBootstrapFileContentEnv = "GRPC_XDS_BOOTSTRAP_CONFIG"
)

var (
	// XDSBootstrapFileName holds the name of the file which contains xDS
	// bootstrap configuration. Users can specify the location of the bootstrap
	// file by setting the environment variable "GRPC_XDS_BOOTSTRAP".
	//
	// When both bootstrap FileName and FileContent are set, FileName is used.
	XDSBootstrapFileName = os.Getenv(XDSBootstrapFileNameEnv)
	// XDSBootstrapFileContent holds the content of the xDS bootstrap
	// configuration. Users can specify the bootstrap config by setting the
	// environment variable "GRPC_XDS_BOOTSTRAP_CONFIG".
	//
	// When both bootstrap FileName and FileContent are set, FileName is used.
	XDSBootstrapFileContent = os.Getenv(XDSBootstrapFileContentEnv)
	// GetCertificateProviderBuilder returns the registered builder for the
	// given name. This is set by package certprovider for use from xDS
	// bootstrap code while parsing certificate provider configs in the
	// bootstrap file.
	GetCertificateProviderBuilder any // func(string) certprovider.Builder

	errNoBootstrapEnvVar = fmt.Errorf("none of the bootstrap environment variables (%q or %q) defined",
		XDSBootstrapFileNameEnv, XDSBootstrapFileContentEnv)
)

// Config provides the xDS client with several key bits of information that it
// requires in its interaction with the management server. The Config is
// initialized from the bootstrap file.
type Config struct {
	// CertProviderConfigs contains a mapping from certificate provider plugin
	// instance names to parsed buildable configs.
	CertProviderConfigs map[string]*certprovider.BuildableConfig
	// NodeProto contains the Node proto to be used in xDS requests. This will be
	// of type *v3corepb.Node.
	NodeProto *v3corepb.Node
}

// NewConfigPartial returns a new instance of Config initialized by reading the
// bootstrap file found at ${GRPC_XDS_BOOTSTRAP} or bootstrap contents specified
// at ${GRPC_XDS_BOOTSTRAP_CONFIG}. If both env vars are set, the former is
// preferred.
//
// Compared to the original `NewConfig()` function in the package
// `google.golang.org/grpc/xds/internal/xdsclient/bootstrap`,
// ([Source]: https://github.com/grpc/grpc-go/blob/v1.57.0/xds/internal/xdsclient/bootstrap/bootstrap.go#L414)
// this partial implementation only reads the `node` and `certificate_provider`
// sections.
//
// We support a credential registration mechanism and only credentials
// registered through that mechanism will be accepted here. See package
// `xds/bootstrap` for details.
func NewConfigPartial() (*Config, error) {
	// Examples of the bootstrap json can be found in the generator tests
	// https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/master/main_test.go.
	data, err := bootstrapConfigFromEnvVariable()
	if err != nil {
		return nil, fmt.Errorf("xds: Failed to read bootstrap config: %w", err)
	}
	return newConfigFromContents(data)
}

func bootstrapConfigFromEnvVariable() ([]byte, error) {
	fName := XDSBootstrapFileName
	fContent := XDSBootstrapFileContent

	// Bootstrap file name has higher priority than bootstrap content.
	if fName != "" {
		// If file name is set
		// - If file not found (or other errors), fail
		// - Otherwise, use the content.
		//
		// Note that even if the content is invalid, we don't fail over to the
		// file content env variable.
		return os.ReadFile(fName)
	}

	if fContent != "" {
		return []byte(fContent), nil
	}

	return nil, errNoBootstrapEnvVar
}

func newConfigFromContents(data []byte) (*Config, error) {
	config := &Config{}

	var jsonData map[string]json.RawMessage
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("xds: failed to parse bootstrap config: %w", err)
	}

	var node *v3corepb.Node
	m := protojson.UnmarshalOptions{
		AllowPartial:   true,
		DiscardUnknown: true,
	}
	for k, v := range jsonData {
		switch k {
		case "node":
			node = &v3corepb.Node{}
			if err := m.Unmarshal(v, node); err != nil {
				return nil, fmt.Errorf("xds: jsonpb.Unmarshal(%v) for field %q failed during bootstrap: %w", string(v), k, err)
			}
		case "certificate_providers":
			var providerInstances map[string]json.RawMessage
			if err := json.Unmarshal(v, &providerInstances); err != nil {
				return nil, fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %w", string(v), k, err)
			}
			configs, err := parseCertificateProviders(providerInstances)
			if err != nil {
				return nil, err
			}
			config.CertProviderConfigs = configs
		}
	}

	if node == nil {
		node = &v3corepb.Node{}
	}
	config.NodeProto = node

	return config, nil
}

func parseCertificateProviders(providerInstances map[string]json.RawMessage) (map[string]*certprovider.BuildableConfig, error) {
	configs := make(map[string]*certprovider.BuildableConfig)
	for instance, data := range providerInstances {
		var nameAndConfig struct {
			PluginName string          `json:"plugin_name"`
			Config     json.RawMessage `json:"config"`
		}
		if err := json.Unmarshal(data, &nameAndConfig); err != nil {
			return nil, fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %w", string(data), instance, err)
		}
		bc := certprovider.NewBuildableConfig(
			nameAndConfig.PluginName,
			[]byte{},
			func(options certprovider.BuildOptions) certprovider.Provider { return nil })
		configs[instance] = bc
	}
	return configs, nil
}
