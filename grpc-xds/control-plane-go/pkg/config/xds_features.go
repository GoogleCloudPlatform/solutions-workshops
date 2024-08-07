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

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds"
)

const (
	xdsFeaturesConfigFile = "xds_features.yaml"
)

var (
	errEBACRequiresDataPlaneMTLS         = errors.New("enableRbac=true requires enableDataPlaneTls=true and requireDataPlaneClientCerts=true")
	errControlPlaneClientCertsRequireTLS = errors.New("requireControlPlaneClientCerts=true requires enableControlPlaneTls=true")
	errDataPlaneClientCertsRequireTLS    = errors.New("requireDataPlaneClientCerts=true requires enableDataPlaneTls=true")
)

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
	if err := validateXDSFeatureFlags(xdsFeatures); err != nil {
		return nil, fmt.Errorf("xDS feature flags validation failed: %w", err)
	}
	logger.V(2).Info("xDS features", "flags", xdsFeatures)
	return &xdsFeatures, err
}

func validateXDSFeatureFlags(xdsFeatures xds.Features) error {
	if xdsFeatures.RequireControlPlaneClientCerts && !xdsFeatures.EnableControlPlaneTLS {
		return errControlPlaneClientCertsRequireTLS
	}
	if xdsFeatures.RequireDataPlaneClientCerts && !xdsFeatures.EnableDataPlaneTLS {
		return errDataPlaneClientCertsRequireTLS
	}
	if xdsFeatures.EnableRBAC && (!xdsFeatures.EnableDataPlaneTLS || !xdsFeatures.RequireDataPlaneClientCerts) {
		return errEBACRequiresDataPlaneMTLS
	}
	return nil
}
