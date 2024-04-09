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
)

const (
	appNameLabelFilepathDownwardAPI = "/etc/podinfo/label-app-name"
)

// AppName returns the value of the `app.kubernetes.io/name` label on this pod.
// This function reads the value from a file in a volume that was mounted using the downward API.
func AppName() (string, error) {
	appNameBytes, err := os.ReadFile(appNameLabelFilepathDownwardAPI)
	if err != nil {
		return "", fmt.Errorf("could not read the value of the app.kubernetes.io/name label from the file %q: %w", appNameLabelFilepathDownwardAPI, err)
	}
	return string(appNameBytes), nil
}
