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
	"net"
	"strings"
)

const (
	kubernetesService = "kubernetes.default.svc"
)

// ClusterDNSDomain returns the Kubernetes cluster's DNS domain, e.g., `cluster.local`.
func ClusterDNSDomain() (string, error) {
	cname, err := net.LookupCNAME(kubernetesService)
	if err != nil {
		return "", fmt.Errorf("could not look up canonical name of %s: %w", kubernetesService, err)
	}
	clusterDNSDomain := strings.TrimPrefix(cname, kubernetesService+".")
	return strings.TrimSuffix(clusterDNSDomain, "."), nil
}
