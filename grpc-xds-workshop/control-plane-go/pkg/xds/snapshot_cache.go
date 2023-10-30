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
	"sync"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"github.com/go-logr/logr"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/logging"
)

var (
	serverListenerNamePrefix = strings.SplitAfter(serverListenerResourceNameTemplate, "=")[0]
)

// SnapshotCache stores snapshots of xDS resources in a delegate cache.
//
// It handles server listener requests by intercepting Listener stream creation, see `CreateWatch()`.
// Server listeners addresses from these requests are kept in a map, keyed by the node hash,
// and with a set of addresses per node hash.
//
// It also handles propagating snapshots to all node hashes in the cache.
type SnapshotCache struct {
	ctx                        context.Context
	logger                     logr.Logger
	delegate                   cachev3.SnapshotCache
	hash                       cachev3.NodeHash
	serverListenerAddresses    map[string]map[EndpointAddress]bool
	serverListenerAddressesMux *sync.RWMutex
}

// NewSnapshotCache creates an xDS resource cache for the provided node hash function.
//
// If `allowPartialRequests` is true, the DiscoveryServer will respond to requests for a resource
// type even if some resources in the snapshot are not named in the request.
func NewSnapshotCache(ctx context.Context, allowPartialRequests bool, hash cachev3.NodeHash) *SnapshotCache {
	return &SnapshotCache{
		ctx:                        ctx,
		logger:                     logging.FromContext(ctx),
		delegate:                   cachev3.NewSnapshotCache(!allowPartialRequests, hash, logging.SnapshotCacheLogger(ctx)),
		hash:                       hash,
		serverListenerAddresses:    make(map[string]map[EndpointAddress]bool),
		serverListenerAddressesMux: &sync.RWMutex{},
	}
}

// SetSnapshot creates a new snapshot for each node hash in the cache, based on the provided snapshot builder,
// with the addition of server listeners and their associated route configurations.
func (c *SnapshotCache) SetSnapshot(ctx context.Context, snapshotBuilder *SnapshotBuilder) error {
	var errs []error
	for _, nodeHash := range c.delegate.GetStatusKeys() {
		serverListenerAddresses := c.getServerListenerAddressesForNode(nodeHash)
		snapshotForNodeHash, err := snapshotBuilder.
			AddServerListenerAddresses(serverListenerAddresses...).
			Build()
		if err != nil {
			errs = append(errs, fmt.Errorf("could not create snapshot with server Listeners for nodeHash=%s: %w", nodeHash, err))
		}
		if err = c.delegate.SetSnapshot(ctx, nodeHash, snapshotForNodeHash); err != nil {
			errs = append(errs, fmt.Errorf("could not set snapshot for nodeHash=%s: %w", nodeHash, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c *SnapshotCache) getServerListenerAddressesForNode(nodeHash string) []EndpointAddress {
	c.serverListenerAddressesMux.RLock()
	defer c.serverListenerAddressesMux.RUnlock()
	serverListenerAddresses := make([]EndpointAddress, len(c.serverListenerAddresses[nodeHash]))
	i := 0
	for address := range c.serverListenerAddresses[nodeHash] {
		serverListenerAddresses[i] = address
		i++
	}
	return serverListenerAddresses
}

// CreateWatch intercepts stream creation before delegating, and if it is a new Listener stream, does the following:
//
//   - Extracts addresses and ports of any server listeners in the request and adds them to the
//     set of known server listener socket addresses for the node hash.
//   - If there is no existing snapshot, or if the request contained new and previously unseen
//     server listener addresses the node hash, creates a new snapshot for that node hash,
//     with the server listeners and associated route configuration.
//
// This solves (in a slightly hacky way) bootstrapping of xDS-enabled gRPC servers.
func (c *SnapshotCache) CreateWatch(request *cachev3.Request, state stream.StreamState, responses chan cachev3.Response) (cancel func()) {
	// TODO: Should also look for the server listener in the resource names of the request.
	if request != nil && len(request.ResourceNames) > 0 && request.GetTypeUrl() == "type.googleapis.com/envoy.config.listener.v3.Listener" {
		nodeHash := c.hash.ID(request.GetNode())
		addressesFromRequest, err := findServerListenerAddresses(request.ResourceNames)
		if err != nil {
			c.logger.Error(err, "Problem encountered when looking for server listener addresses in new Listener stream request", "nodeHash", nodeHash)
			return func() {}
		}
		addressesAdded := c.addServerListenerAddressForNode(nodeHash, addressesFromRequest)
		oldSnapshot, err := c.delegate.GetSnapshot(nodeHash)
		if err != nil || addressesAdded {
			if err := c.createNewSnapshotForNode(nodeHash, oldSnapshot, addressesFromRequest); err != nil {
				c.logger.Error(err, "Could not set new xDS resource snapshot", "nodeHash", nodeHash, "oldSnapshot", oldSnapshot, "serverListenerAddressesFromRequest", addressesFromRequest)
				return func() {}
			}
		}
	}
	return c.delegate.CreateWatch(request, state, responses)
}

// createNewSnapshotForNode sets a new snapshot for the provided `nodeHash`,
// based on the previous `oldSnapshot` (can be nil), and a slice of new server Listener addresses.
func (c *SnapshotCache) createNewSnapshotForNode(nodeHash string, oldSnapshot cachev3.ResourceSnapshot, newServerListenerAddresses []EndpointAddress) error {
	c.logger.Info("Creating a new snapshot", "nodeHash", nodeHash, "serverListenerAddressesFromRequest", newServerListenerAddresses)
	newSnapshot, err := NewSnapshotBuilder().
		AddSnapshot(oldSnapshot).
		AddServerListenerAddresses(newServerListenerAddresses...).
		Build()
	if err != nil {
		return fmt.Errorf("could not create new xDS resource snapshot for nodeHash=%s: %w", nodeHash, err)
	}
	if err := c.delegate.SetSnapshot(c.ctx, nodeHash, newSnapshot); err != nil {
		return fmt.Errorf("could not set new xDS resource snapshot for nodeHash=%s: %w", nodeHash, err)
	}
	return nil
}

// findServerListenerAddresses looks for server Listener names in the provided
// slice and extracts the address and port for each server Listener found.
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
				host: host,
				port: uint32(port),
			})
		}
	}
	return addresses, nil
}

// addServerListenerAddressForNode returns true if the provided `newAddresses` slice
// contains server Listener addresses not previously recorded for the provided `nodeHash`.
func (c *SnapshotCache) addServerListenerAddressForNode(nodeHash string, newAddresses []EndpointAddress) bool {
	c.serverListenerAddressesMux.Lock()
	defer c.serverListenerAddressesMux.Unlock()
	addressesForNode := c.serverListenerAddresses[nodeHash]
	if addressesForNode == nil {
		addressesForNode = make(map[EndpointAddress]bool, len(newAddresses))
		c.serverListenerAddresses[nodeHash] = addressesForNode
	}
	numAddressesBefore := len(addressesForNode)
	for _, address := range newAddresses {
		addressesForNode[address] = true
	}
	numAddressesAfter := len(addressesForNode)
	// Since we never remove server Listener addresses for a node hash,
	// we can just check if the set grew.
	return numAddressesAfter > numAddressesBefore
}

// CreateDeltaWatch just delegates, since gRPC does not support delta/incremental xDS currently.
// TODO: Handle request for gRPC server Listeners once gRPC implementation support delta/incremental xDS.
func (c *SnapshotCache) CreateDeltaWatch(request *cachev3.DeltaRequest, state stream.StreamState, responses chan cachev3.DeltaResponse) (cancel func()) {
	return c.delegate.CreateDeltaWatch(request, state, responses)
}

func (c *SnapshotCache) Fetch(ctx context.Context, request *cachev3.Request) (cachev3.Response, error) {
	return c.delegate.Fetch(ctx, request)
}
