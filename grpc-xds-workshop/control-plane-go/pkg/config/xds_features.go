// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/xds"
)

const (
	xdsFeaturesConfigFile = "xds_features.yaml"
)

var errDataPlaneClientCertsRequireTLS = errors.New("enableDataPlaneTls=true is required when requireDataPlaneClientCerts=true")

func XDSFeatures(logger logr.Logger) (*xds.Features, error) {
	configDir, exists := os.LookupEnv("CONFIG_DIR")
	if !exists {
		configDir = defaultConfigDir
	}
	xdsFeaturesConfigFilePath := filepath.Join(configDir, xdsFeaturesConfigFile)
	logger.V(4).Info("Loading xDS feature flags", "filepath", xdsFeaturesConfigFilePath)
	yamlBytes, err := os.ReadFile(xdsFeaturesConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read xDS feature flags from file %s: %w", xdsFeaturesConfigFilePath, err)
	}
	var xdsFeatures xds.Features
	err = yaml.Unmarshal(yamlBytes, &xdsFeatures)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshall xDS feature flags YAML file contents [%s]: %w", yamlBytes, err)
	}
	if err := validateXDSFeatureFlags(logger, xdsFeatures); err != nil {
		return nil, fmt.Errorf("xDS feature flags validation failed: %w", err)
	}
	logger.V(2).Info("xDS features", "flags", xdsFeatures)
	return &xdsFeatures, err
}

func validateXDSFeatureFlags(logger logr.Logger, xdsFeatures xds.Features) error {
	if !xdsFeatures.EnableDataPlaneTLS && xdsFeatures.RequireDataPlaneClientCerts {
		return errDataPlaneClientCertsRequireTLS
	}
	if xdsFeatures.ServerListenerUsesRDS {
		logger.V(1).Info("Warning: gRPC-Go as of v1.60.1 does not support RouteConfiguration via RDS for server Listeners, see https://github.com/grpc/grpc-go/issues/6788")
	}
	return nil
}
