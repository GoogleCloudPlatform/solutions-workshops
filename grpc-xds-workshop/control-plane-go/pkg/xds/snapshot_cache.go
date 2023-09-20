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

package xds

import (
	"context"
	"errors"
	"fmt"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"github.com/go-logr/logr"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/logging"
)

const (
	envoyHTTPConnectionManagerName = "envoy.http_connection_manager"
	envoyFilterHTTPFaultName       = "envoy.filters.http.fault"
	envoyFilterHTTPRouterName      = "envoy.filters.http.router"
	// serverListenerAddressIPv4 is the IPv4 listener address for xDS clients that serve gRPC services.
	serverListenerAddressIPv4 = "0.0.0.0"
	// serverListenerAddressIPv6 is the IPv6 listener address for xDS clients that serve gRPC services.
	serverListenerAddressIPv6 = "[::]"
	// serverListenerPort is the listener port for the server listener.
	// In the current implementations, gRPC server Pods must listen on this hardcoded port for the services they provide.
	serverListenerPort = 50051
	// serverListenerResourceNameTemplate uses the address and port above. Must match the template in the gRPC xDS bootstrap file, see
	// [gRFC A36: xDS-Enabled Servers]: https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md#xds-protocol
	// Using the sample name from the gRPC-Go unit tests, but this is not important.
	serverListenerResourceNameTemplate = "grpc/server?xds.resource.listening_address=%s"
	// serverListenerRouteConfigName uses the same route configuration serviceName as Traffic Director, but this is not important.
	serverListenerRouteConfigName = "inbound|default_inbound_config-50051"
)

// SnapshotCache stores snapshots of xDS resources in a delegate cache.
//
// It handles the initial server listener and route configuration request by intercepting
// Listener stream creation, see `CreateWatch()`.
//
// It also handles propagating snapshots to all node groups in the cache. This is currently
// done naively, by using the same snapshot for all node groups.
type SnapshotCache struct {
	logger   logr.Logger
	delegate cachev3.SnapshotCache
	hash     cachev3.NodeHash
}

// NewSnapshotCache creates an xDS resource cache for the provided node group mapping (`hash`).
//
// If `allowPartialRequests` is true, the DiscoveryServer will respond to requests for a resource
// type even thouh not all resources of that type in the snapshot are named in the request.
func NewSnapshotCache(ctx context.Context, allowPartialRequests bool, hash cachev3.NodeHash) *SnapshotCache {
	return &SnapshotCache{
		logger:   logging.FromContext(ctx),
		delegate: cachev3.NewSnapshotCache(!allowPartialRequests, hash, logging.SnapshotCacheLogger(ctx)),
		hash:     hash,
	}
}

// SetSnapshot naively sets the provided snapshot for all node groups in the cache.
func (c *SnapshotCache) SetSnapshot(ctx context.Context, snapshot cachev3.ResourceSnapshot) error {
	var errs []error
	for _, nodeID := range c.delegate.GetStatusKeys() {
		err := c.delegate.SetSnapshot(ctx, nodeID, snapshot)
		if err != nil {
			errs = append(errs, fmt.Errorf("could not set snapshot for nodeID=%s: %w", nodeID, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// CreateWatch intercepts stream creation, and if it is a new Listener stream,
// creates a snapshot with the server listener and associated route configuration.
//
// This solves (in a slightly hacky way) bootstrapping of xDS-enabled gRPC servers.
func (c *SnapshotCache) CreateWatch(request *cachev3.Request, state stream.StreamState, responses chan cachev3.Response) (cancel func()) {
	// TODO: Should also look for the server listener in the resource names of the request.
	if request != nil && request.GetTypeUrl() == "type.googleapis.com/envoy.config.listener.v3.Listener" {
		nodeID := c.hash.ID(request.GetNode())
		_, err := c.delegate.GetSnapshot(nodeID)
		if err != nil {
			c.logger.Info("Missing snapshot, creating a snapshot with server listener and route configuration", "nodeID", nodeID)
			if err := c.createSnapshotWithServerListener(nodeID); err != nil {
				c.logger.Error(err, "Could not create xDS resource snapshot", "nodeID", nodeID)
				return func() {}
			}
		}
	}
	return c.delegate.CreateWatch(request, state, responses)
}

func (c *SnapshotCache) createSnapshotWithServerListener(node string) error {
	c.logger.V(1).Info("Missing snapshot, creating a snapshot with server listener and route configuration", "node", node)
	snapshot, err := NewSnapshotBuilder().Build()
	if err != nil {
		return err
	}
	err = c.delegate.SetSnapshot(context.Background(), node, snapshot)
	if err != nil {
		return err
	}
	return nil
}

// CreateDeltaWatch just delegates, since delta xDS is not supported by this control plane implementation.
func (c *SnapshotCache) CreateDeltaWatch(request *cachev3.DeltaRequest, state stream.StreamState, responses chan cachev3.DeltaResponse) (cancel func()) {
	return c.delegate.CreateDeltaWatch(request, state, responses)
}

func (c *SnapshotCache) Fetch(ctx context.Context, request *cachev3.Request) (cachev3.Response, error) {
	return c.delegate.Fetch(ctx, request)
}
