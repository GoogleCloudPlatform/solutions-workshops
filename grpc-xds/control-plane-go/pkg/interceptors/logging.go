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

package interceptors

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"google.golang.org/grpc"
	"google.golang.org/grpc/channelz/grpc_channelz_v1"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// verbosity https://github.com/kubernetes/community/blob/1a09f121536ddb84c1429c88fbb3978d6c5e2dd0/contributors/devel/sig-instrumentation/logging.md#what-method-to-use
const (
	debugVerbosity = 4
	infoVerbosity  = 2
	warnVerbosity  = 1
	errorVerbosity = 0

	interceptorLoggerCallDepth = 3
)

var (
	// gRPC services to exclude from logging.
	excludedServices = map[string]bool{
		grpc_channelz_v1.Channelz_ServiceDesc.ServiceName:                true,
		grpc_health_v1.Health_ServiceDesc.ServiceName:                    true,
		grpc_reflection_v1.ServerReflection_ServiceDesc.ServiceName:      true,
		grpc_reflection_v1alpha.ServerReflection_ServiceDesc.ServiceName: true,
		"envoy.service.status.v2.ClientStatusDiscoveryService":           true, // not exported
		"envoy.service.status.v3.ClientStatusDiscoveryService":           true, // not exported
	}

	loggingOpts = []logging.Option{
		logging.WithLogOnEvents(
			logging.PayloadReceived,
			logging.PayloadSent),
	}
)

func StreamServerLogging(logger logr.Logger) grpc.StreamServerInterceptor {
	loggingInterceptor := logging.StreamServerInterceptor(interceptorLogger(logger), loggingOpts...)
	return selector.StreamServerInterceptor(loggingInterceptor, selector.MatchFunc(selectorFunc))
}

func UnaryServerLogging(logger logr.Logger) grpc.UnaryServerInterceptor {
	loggingInterceptor := logging.UnaryServerInterceptor(interceptorLogger(logger), loggingOpts...)
	return selector.UnaryServerInterceptor(loggingInterceptor, selector.MatchFunc(selectorFunc))
}

func selectorFunc(_ context.Context, callMeta interceptors.CallMeta) bool {
	return !excludedServices[callMeta.Service]
}

// interceptorLogger adapts logr logger to interceptor logger.
//
// This function also marshals any `fields` of type `proto.Message` into
// pretty-printed multi-line JSON strings, to make log tailing easier during
// development. This approach is not recommended for production environments.
func interceptorLogger(l logr.Logger) logging.Logger {
	return logging.LoggerFunc(func(_ context.Context, lvl logging.Level, msg string, fields ...any) {
		if fields == nil {
			fields = make([]any, 0)
		}
		protoMarshalOptions := protojson.MarshalOptions{
			Multiline:    true,
			Indent:       "  ",
			AllowPartial: true,
		}
		for i, field := range fields {
			if message, ok := field.(proto.Message); ok {
				messageJSONBytes, err := protoMarshalOptions.Marshal(message)
				if err == nil {
					fields[i] = string(messageJSONBytes)
				}
			}
		}
		l := l.WithCallDepth(interceptorLoggerCallDepth).WithValues(fields...)
		switch lvl {
		case logging.LevelDebug:
			l.V(debugVerbosity).Info(msg)
		case logging.LevelInfo:
			l.V(infoVerbosity).Info(msg)
		case logging.LevelWarn:
			l.V(warnVerbosity).Info(msg)
		case logging.LevelError:
			l.V(errorVerbosity).Error(nil, msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}
