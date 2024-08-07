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

package xds

// Features of the xDS control plane that can be enabled and disabled via a config file.
type Features struct {
	EnableControlPlaneTLS          bool `yaml:"enableControlPlaneTls"`
	RequireControlPlaneClientCerts bool `yaml:"requireControlPlaneClientCerts"`
	EnableDataPlaneTLS             bool `yaml:"enableDataPlaneTls"`
	RequireDataPlaneClientCerts    bool `yaml:"requireDataPlaneClientCerts"`
	EnableRBAC                     bool `yaml:"enableRbac"`
	EnableFederation               bool `yaml:"enableFederation"`
}
