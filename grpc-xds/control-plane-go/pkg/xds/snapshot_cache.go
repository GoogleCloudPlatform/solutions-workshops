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
	"net"
	"strconv"
	"strings"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	streamv3 "github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"github.com/go-logr/logr"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/applications"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/logging"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/eds"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds/lds"
)

// Server listener resource names typically follow the template `grpc/server?xds.resource.listening_address=%s`.
// serverListenerNamePrefix is the part up to and including the `=` sign.
var serverListenerNamePrefix = strings.SplitAfter(lds.GRPCServerListenerResourceNameTemplate, "=")[0]

// SnapshotCache stores snapshots of xDS resources in a delegate cache.
//
// It handles server listener requests by intercepting Listener stream creation, see `CreateWatch()`.
// Server listeners addresses from these requests are kept in a map, keyed by the node hash,
// and with a set of addresses per node hash.
//
// It also handles propagating snapshots to all node hashes in the cache.
type SnapshotCache struct {
	ctx    context.Context
	logger logr.Logger
	// delegate is the actual xDS resource cache.
	delegate cachev3.SnapshotCache
	// hash is the function to determine the cache key (`nodeHash`) for nodes.
	hash cachev3.NodeHash
	// localityPriorityMapper constructs a priority map for localities, to be used in EDS ClusterLoadAssignment resources.
	localityPriorityMapper eds.LocalityPriorityMapper
	// appsCache stores the most recent gRPC application configuration information from k8s cluster EndpointSlices.
	// The appsCache is used to populate new entries (previously unseen `nodeHash`es) in the xDS resource snapshot cache,
	// so that the new subscribers don't have to wait for an EndpointSlice update before they can receive xDS resources.
	appsCache *applications.ApplicationCache
	// grpcServerListenerCache stores known server listener names for each snapshot cache key (`nodeHash`).
	// These names are captured when new Listener streams are created, see `CreateWatch()`.
	// The server listener names are added to xDS resource snapshots, to be included in LDS responded for xDS-enabled gRPC servers.
	grpcServerListenerCache *GRPCServerListenerCache
	// features contains flags to enable and disable xDS features, e.g., mTLS.
	features *Features
	// authority is the authority name of this control plane for xDS federation.
	authority string
}

var _ cachev3.Cache = &SnapshotCache{}

// NewSnapshotCache creates an xDS resource cache for the provided node hash function.
//
// If `allowPartialRequests` is true, the DiscoveryServer will respond to requests for a resource
// type even if some resources in the snapshot are not named in the request.
func NewSnapshotCache(ctx context.Context, allowPartialRequests bool, hash cachev3.NodeHash, localityPriorityMapper eds.LocalityPriorityMapper, features *Features, authority string) *SnapshotCache {
	return &SnapshotCache{
		ctx:                     ctx,
		logger:                  logging.FromContext(ctx),
		delegate:                cachev3.NewSnapshotCache(!allowPartialRequests, hash, logging.SnapshotCacheLogger(ctx)),
		hash:                    hash,
		localityPriorityMapper:  localityPriorityMapper,
		appsCache:               applications.NewApplicationCache(),
		grpcServerListenerCache: NewGRPCServerListenerCache(),
		features:                features,
		authority:               authority,
	}
}

// CreateWatch intercepts stream creation before delegating, and if it is a request for Listener
// (LDS) resources stream, does the following:
//
//   - Extracts addresses and ports of any server listeners in the request and adds them to the
//     set of known server listener socket addresses for the node hash.
//   - If there is no existing snapshot, or if the request contained new and previously unseen
//     server listener addresses the node hash, creates a new snapshot for that node hash,
//     with the server listeners and associated route configuration.
//
// This solves bootstrapping of xDS resources snapshots for xDS-enabled gRPC servers and
// Envoy proxy instances that fetch configuration dynamically using ADS.
func (c *SnapshotCache) CreateWatch(request *cachev3.Request, state streamv3.StreamState, responses chan cachev3.Response) (cancel func()) {
	if isListenerRequest(request) {
		c.logger.Info("CreateWatch",
			"typeUrl", request.TypeUrl,
			"resourceNames", request.ResourceNames,
			"node.cluster", request.Node.Cluster,
			"node.user_agent_name", request.Node.UserAgentName,
			"node.id", request.Node.Id)
		nodeHash := c.hash.ID(request.GetNode())
		addressesFromRequest, err := findServerListenerAddresses(request.ResourceNames)
		if err != nil {
			c.logger.Error(err, "Problem encountered when looking for server listener addresses in new Listener stream request", "nodeHash", nodeHash)
			return func() {}
		}
		changes := c.grpcServerListenerCache.Add(nodeHash, addressesFromRequest)
		existingSnapshot, err := c.delegate.GetSnapshot(nodeHash)
		if err != nil || existingSnapshot == nil || changes {
			apps := c.appsCache.GetAll()
			if err := c.createNewSnapshot(nodeHash, apps); err != nil {
				c.logger.Error(err, "Could not set new xDS resource snapshot", "nodeHash", nodeHash, "apps", apps)
				return func() {}
			}
		}
	}
	return c.delegate.CreateWatch(request, state, responses)
}

// UpdateResources creates a new snapshot for each node hash in the cache,
// based on the provided gRPC application configuration,
// with the addition of server listeners and their associated route configurations.
func (c *SnapshotCache) UpdateResources(_ context.Context, logger logr.Logger, kubecontextName string, namespace string, updatedApps []applications.Application) error {
	var errs []error
	changed := c.appsCache.Put(kubecontextName, namespace, updatedApps)
	if !changed {
		logger.V(2).Info("No application updates, so not generating new xDS resource snapshots")
		return nil
	}
	apps := c.appsCache.GetAll()
	logger.V(2).Info("Application updates, generating new xDS resource snapshots", "apps", apps)
	for _, nodeHash := range c.delegate.GetStatusKeys() {
		if err := c.createNewSnapshot(nodeHash, apps); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// createNewSnapshot sets a new snapshot for the provided `nodeHash` and gRPC application configuration.
func (c *SnapshotCache) createNewSnapshot(nodeHash string, apps []applications.Application) error {
	c.logger.Info("Creating a new snapshot", "nodeHash", nodeHash, "apps", apps)
	snapshotBuilder, err := NewSnapshotBuilder(nodeHash, c.localityPriorityMapper, c.features, c.authority).AddGRPCApplications(apps)
	if err != nil {
		return fmt.Errorf("could not create xDS resource snapshot builder for nodeHash=%s: %w", nodeHash, err)
	}
	snapshot, err := snapshotBuilder.
		AddGRPCServerListenerAddresses(c.grpcServerListenerCache.Get(nodeHash)).
		Build()
	if err != nil {
		return fmt.Errorf("could not create new xDS resource snapshot for nodeHash=%s: %w", nodeHash, err)
	}
	if err := c.delegate.SetSnapshot(c.ctx, nodeHash, snapshot); err != nil {
		return fmt.Errorf("could not set new xDS resource snapshot for nodeHash=%s: %w", nodeHash, err)
	}
	return nil
}

// isListenerRequest determines if the request is a request for Listener (LDS) resources.
func isListenerRequest(request *cachev3.Request) bool {
	return request != nil &&
		(len(request.ResourceNames) > 0 || request.Node.UserAgentName == "envoy") &&
		request.GetTypeUrl() == resourcev3.ListenerType
}

// findServerListenerAddresses looks for server Listener names in the provided
// slice and extracts the address and port for each server Listener found.
// TODO: Handle xDS federation server Listener names using `xdstp://` names,
// e.g., "xdstp://xds-authority.example.com/envoy.config.listener.v3.Listener/grpc/server/%s"
func findServerListenerAddresses(names []string) ([]EndpointAddress, error) {
	var addresses []EndpointAddress
	for _, name := range names {
		if strings.HasPrefix(name, serverListenerNamePrefix) && len(name) > len(serverListenerNamePrefix) {
			hostPort := strings.SplitAfter(name, serverListenerNamePrefix)[1]
			host, portStr, err := net.SplitHostPort(hostPort)
			if err != nil {
				return nil, fmt.Errorf("could not extract host and port from server Listener name=%s: %w", name, err)
			}
			port, err := strconv.ParseUint(portStr, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("could not extract port from server Listener name: %w", err)
			}
			addresses = append(addresses, EndpointAddress{
				Host: host,
				Port: uint32(port),
			})
		}
	}
	return addresses, nil
}

// CreateDeltaWatch just delegates, since gRPC does not support delta/incremental xDS currently.
// TODO: Handle request for gRPC server Listeners once gRPC implementation support delta/incremental xDS.
func (c *SnapshotCache) CreateDeltaWatch(request *cachev3.DeltaRequest, state streamv3.StreamState, responses chan cachev3.DeltaResponse) (cancel func()) {
	return c.delegate.CreateDeltaWatch(request, state, responses)
}

func (c *SnapshotCache) Fetch(ctx context.Context, request *cachev3.Request) (cachev3.Response, error) {
	return c.delegate.Fetch(ctx, request)
}
