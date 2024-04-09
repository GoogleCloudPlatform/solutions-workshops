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

	"github.com/go-logr/logr"
)

// AuthorityName returns the expected authority name of this control plane.
// The authority name is used in xDS federation, where xDS clients can specify
// the authority of an xDS resource.
//
// The authority name format assumed in this control plane implementation is of the format
// `[app-name].[namespace].svc.[k8s-dns-cluster-domain]`, e.g.,
// `control-plane.xds.svc.cluster.local`.
// xDS clients must use this format in the `authorities` section of their gRPC xDS bootstrap configuration.
//
// See
// [xRFC TP1](https://github.com/cncf/xds/blob/70da609f752ed4544772f144411161d41798f07e/proposals/TP1-xds-transport-next.md#federation)
// and
// [gRFC A47](https://github.com/grpc/proposal/blob/e85c66e48348867937688d89117bad3dcaa6f4f5/A47-xds-federation.md).
func AuthorityName(logger logr.Logger) (string, error) {
	appName, err := AppName()
	if err != nil {
		return "", fmt.Errorf("could not determine app name for xDS control plane authority name: %w", err)
	}
	namespace, err := Namespace(logger)
	if err != nil {
		return "", fmt.Errorf("could not determine namespace for xDS control plane authority name: %w", err)
	}
	clusterDNSDomain, err := ClusterDNSDomain()
	if err != nil {
		return "", fmt.Errorf("could not determine cluster DNS domain for xDS control plane authority name: %w", err)
	}
	return fmt.Sprintf("%s.%s.svc.%s", appName, namespace, clusterDNSDomain), nil
}
