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

package lds

import (
	"fmt"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	envoyFilterHTTPFaultName = "envoy.filters.http.fault"
)

// CreateAPIListener returns an LDS API listener
//
// [gRFC A27]: https://github.com/grpc/proposal/blob/master/A27-xds-global-load-balancing.md#listener-proto
// [Reference]: https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/listener/v3/api_listener.proto
func CreateAPIListener(name string, routeConfigurationName string) (*listenerv3.Listener, error) {
	httpConnectionManager, err := createUpstreamHTTPConnectionManager(routeConfigurationName, name)
	if err != nil {
		return nil, fmt.Errorf("could not create HttpConnectionManager for LDS API Listener: %w", err)
	}
	anyWrappedHTTPConnectionManager, err := anypb.New(httpConnectionManager)
	if err != nil {
		return nil, fmt.Errorf("could not marshall HttpConnectionManager +%v into Any instance: %w", httpConnectionManager, err)
	}
	return &listenerv3.Listener{
		Name: name,
		ApiListener: &listenerv3.ApiListener{
			ApiListener: anyWrappedHTTPConnectionManager,
		},
	}, nil
}
