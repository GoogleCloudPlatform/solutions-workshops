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

package config

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
)

const (
	namespaceFilepathDownwardAPI    = "/etc/podinfo/namespace"
	namespaceFilepathServiceAccount = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// Namespace returns the Kubernetes namespace of this pod.
// This function first looks for a file in a volume that was mounted using the downward API.
// If that doesn't exist, it looks for the `namespace` file in the `serviceaccount` directory.
// If neither of those files exist, this function returns an error.
func Namespace(logger logr.Logger) (string, error) {
	namespaceBytes, err := os.ReadFile(namespaceFilepathDownwardAPI)
	if err == nil {
		return string(namespaceBytes), nil
	}
	logger.Error(err, "Could not read pod namespace from expected downward API volume, looking in service account directory instead", "path", namespaceFilepathDownwardAPI)
	namespaceBytes, err = os.ReadFile(namespaceFilepathServiceAccount)
	if err == nil {
		return string(namespaceBytes), nil
	}
	return "", fmt.Errorf("could not determine the pod namespace from %q or %q: %w", namespaceFilepathDownwardAPI, namespaceFilepathServiceAccount, err)
}
