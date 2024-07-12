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

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	rbacv3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	faultv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/fault/v3"
	rbacfilterv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	http_connection_managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	EnvoyFilterHTTPRBACName   = "envoy.filters.http.rbac"
	envoyFilterHTTPFaultName  = "envoy.filters.http.fault"
	envoyFilterHTTPRouterName = "envoy.filters.http.router"
)

// createHTTPConnectionManagerForSocketListener returns a HttpConnectionManager to be
// used with LDS Listeners for gRPC servers and Envoy proxy instances.
func createHTTPConnectionManagerForSocketListener(routeConfigurationName string, statPrefix string, enableRBAC bool) (*http_connection_managerv3.HttpConnectionManager, error) {
	routerFilterConfig, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("could not marshall Router HTTP filter into Any instance: %w", err)
	}
	httpConnectionManager := http_connection_managerv3.HttpConnectionManager{
		CodecType:  http_connection_managerv3.HttpConnectionManager_AUTO,
		StatPrefix: statPrefix,
		HttpFilters: []*http_connection_managerv3.HttpFilter{
			{
				// Router must be the last HTTP filter.
				Name: envoyFilterHTTPRouterName,
				ConfigType: &http_connection_managerv3.HttpFilter_TypedConfig{
					TypedConfig: routerFilterConfig,
				},
			},
		},
		RouteSpecifier: &http_connection_managerv3.HttpConnectionManager_Rds{
			Rds: &http_connection_managerv3.Rds{
				ConfigSource: &corev3.ConfigSource{
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
						Ads: &corev3.AggregatedConfigSource{},
					},
					ResourceApiVersion: corev3.ApiVersion_V3,
				},
				RouteConfigName: routeConfigurationName,
			},
		},
		ForwardClientCertDetails: http_connection_managerv3.HttpConnectionManager_APPEND_FORWARD,
		SetCurrentClientCertDetails: &http_connection_managerv3.HttpConnectionManager_SetCurrentClientCertDetails{
			Subject: wrapperspb.Bool(true),
			Dns:     true,
			Uri:     true,
		},
		UpgradeConfigs: []*http_connection_managerv3.HttpConnectionManager_UpgradeConfig{
			{
				UpgradeType: "websocket",
			},
		},
	}

	if enableRBAC {
		rbacFilterTypedConfig, err := anypb.New(&rbacfilterv3.RBAC{
			Rules: &rbacv3.RBAC{}, // Present and empty `Rules` mean DENY all. Override per route.
		})
		if err != nil {
			return nil, fmt.Errorf("could not marshall RBAC HTTP filter typedConfig into Any instance: %w", err)
		}
		// Prepend RBAC HTTP filter. Not append, as Router must be the last HTTP filter.
		httpConnectionManager.HttpFilters = append([]*http_connection_managerv3.HttpFilter{
			{
				Name: EnvoyFilterHTTPRBACName,
				ConfigType: &http_connection_managerv3.HttpFilter_TypedConfig{
					TypedConfig: rbacFilterTypedConfig,
				},
			},
		}, httpConnectionManager.HttpFilters...)
	}

	return &httpConnectionManager, nil
}

// createHTTPConnectionManagerForAPIListener returns a HttpConnectionManager to be
// used with LDS API Listeners for gRPC clients.
func createHTTPConnectionManagerForAPIListener(routeConfigurationName string, statPrefix string) (*http_connection_managerv3.HttpConnectionManager, error) {
	httpFaultFilterConfig, err := anypb.New(&faultv3.HTTPFault{})
	if err != nil {
		return nil, fmt.Errorf("could not marshall HTTPFault HTTP filter into Any instance: %w", err)
	}
	routerFilterConfig, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("could not marshall Router HTTP filter into Any instance: %w", err)
	}
	httpConnectionManager := http_connection_managerv3.HttpConnectionManager{
		CodecType:  http_connection_managerv3.HttpConnectionManager_AUTO,
		StatPrefix: statPrefix,
		HttpFilters: []*http_connection_managerv3.HttpFilter{
			{
				// Enable client-side fault injection.
				Name: envoyFilterHTTPFaultName,
				ConfigType: &http_connection_managerv3.HttpFilter_TypedConfig{
					TypedConfig: httpFaultFilterConfig,
				},
			},
			{
				// Router must be the last HTTP filter.
				Name: envoyFilterHTTPRouterName,
				ConfigType: &http_connection_managerv3.HttpFilter_TypedConfig{
					TypedConfig: routerFilterConfig,
				},
			},
		},
		RouteSpecifier: &http_connection_managerv3.HttpConnectionManager_Rds{
			Rds: &http_connection_managerv3.Rds{
				ConfigSource: &corev3.ConfigSource{
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
						Ads: &corev3.AggregatedConfigSource{},
					},
					ResourceApiVersion: corev3.ApiVersion_V3,
				},
				RouteConfigName: routeConfigurationName,
			},
		},
	}
	return &httpConnectionManager, nil
}
