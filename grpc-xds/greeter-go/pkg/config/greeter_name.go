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
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/go-logr/logr"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/logging"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/xdsclient/bootstrap"
)

var errNoLocality = errors.New("no locality information in the gRPC xDS bootstrap configuration")

// GreeterName is constructed from the host name and the zone name.
// The zone name is looked up from the.
func GreeterName(ctx context.Context) string {
	logger := logging.FromContext(ctx)
	zone, err := zoneFromGRPCXDSBootstrapFile()
	if err != nil || zone == "" {
		zone, err = zoneFromGCPMetadataServer(logger)
		if err != nil {
			logger.Error(err, "Could not determine the zone from the GCP metadata server")
		}
	}
	return fmt.Sprintf("%s(%s)", hostname(logger), zone)
}

// hostname returns the host name, or a generated name if there is a problem looking up the host name.
/* #nosec G404 -- Weak random number generator is fine here. */
func hostname(logger logr.Logger) string {
	hostname, err := os.Hostname()
	if err != nil {
		logger.Error(err, "Could not determine the host name, using a generated name")
		rand.New(rand.NewSource(time.Now().UnixNano()))
		return fmt.Sprintf("node-%03d", rand.Int()%100)
	}
	return hostname
}

// zoneFromGRPCXDSBootstrapFile returns the zone name of the Kubernetes cluster
// node where this Pod is scheduled, by parsing the locality information in
// the gRPC xDS bootstrap file or config.
func zoneFromGRPCXDSBootstrapFile() (string, error) {
	bootstrapConfig, err := bootstrap.NewConfigPartial()
	if err != nil {
		return "", fmt.Errorf("could not parse the gRPC xDS bootstrap configuration: %w", err)
	}
	if bootstrapConfig.NodeProto.Locality == nil {
		return "", errNoLocality
	}
	return bootstrapConfig.NodeProto.Locality.Zone, nil
}

// zoneFromGCPMetadataServer returns the zone name of the Kubernetes cluster
// node where this Pod is scheduled, by querying the Google Kubernetes Engine
// or Compute Engine
// [metadata server]: https://cloud.google.com/compute/docs/metadata/overview
func zoneFromGCPMetadataServer(logger logr.Logger) (string, error) {
	zoneWithProjectNumber, err := metadata.Zone()
	if err != nil {
		return "", fmt.Errorf("could not look up the zone from the GCP metadata server: %w", err)
	}
	// zoneWithProjectNumber format is `projects/[PROJECT_NUMBER]/zones/[ZONE]`.
	zoneSplit := strings.Split(zoneWithProjectNumber, "/")
	if len(zoneSplit) != 4 {
		logger.Error(nil, "Unexpected zone format from the GCP metadata server, using it verbatim", "zone", zoneWithProjectNumber)
		return zoneWithProjectNumber, nil
	}
	return zoneSplit[3], nil
}
