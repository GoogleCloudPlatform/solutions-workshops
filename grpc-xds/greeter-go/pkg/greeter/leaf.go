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

package greeter

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	helloworldpb "google.golang.org/grpc/examples/helloworld/helloworld"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/logging"
)

// leafService implements helloworld.Greeter.
type leafService struct {
	helloworldpb.UnimplementedGreeterServer
	logger logr.Logger
	name   string
}

func NewLeafService(ctx context.Context, name string) helloworldpb.GreeterServer {
	return &leafService{
		logger: logging.FromContext(ctx),
		name:   name,
	}
}

func (s *leafService) SayHello(_ context.Context, request *helloworldpb.HelloRequest) (*helloworldpb.HelloReply, error) {
	s.logger.V(2).Info("Received request, returning greeting", "name", request.Name)
	return &helloworldpb.HelloReply{Message: fmt.Sprintf("Hello %s, from %s", request.Name, s.name)}, nil
}
