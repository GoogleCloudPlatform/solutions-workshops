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
	"fmt"
	"os"
	"strconv"
)

const (
	defaultServingPort = 50051
	defaultHealthPort  = 50052
	servingPortEnvVar  = "PORT"
	healthPortEnvVar   = "HEALTH_PORT"
)

func ServingPort() (int, error) {
	port := defaultServingPort
	if portEnv, exists := os.LookupEnv(servingPortEnvVar); exists {
		var err error
		port, err = strconv.Atoi(portEnv)
		if err != nil {
			return 0, fmt.Errorf("could not convert environment variable value %s=%s to integer: %w", servingPortEnvVar, portEnv, err)
		}
	}
	return port, nil
}

func HealthPort() (int, error) {
	port := defaultHealthPort
	if portEnv, exists := os.LookupEnv(healthPortEnvVar); exists {
		var err error
		port, err = strconv.Atoi(portEnv)
		if err != nil {
			return 0, fmt.Errorf("could not convert environment variable value %s=%s to integer: %w", healthPortEnvVar, portEnv, err)
		}
	}
	return port, nil
}
