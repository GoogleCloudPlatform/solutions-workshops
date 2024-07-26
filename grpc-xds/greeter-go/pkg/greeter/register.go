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

package greeter

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	helloworldpb "google.golang.org/grpc/examples/helloworld/helloworld"
)

// RegisterServer registers the Greeter gRPC service to a server.
func RegisterServer(ctx context.Context, logger logr.Logger, greeterName string, nextHop string, server grpc.ServiceRegistrar) error {
	var greeterService helloworldpb.GreeterServer
	if nextHop == "" {
		logger.V(1).Info("Adding leaf Greeter service, as NEXT_HOP is not provided")
		greeterService = NewLeafService(ctx, greeterName)
	} else {
		logger.V(1).Info("Adding intermediary Greeter service", "NEXT_HOP", nextHop)
		greeterClient, err := NewClient(ctx, nextHop)
		if err != nil {
			return fmt.Errorf("could not create greeter client %w", err)
		}
		greeterService = NewIntermediaryService(ctx, greeterName, greeterClient)
	}
	helloworldpb.RegisterGreeterServer(server, greeterService)
	return nil
}
