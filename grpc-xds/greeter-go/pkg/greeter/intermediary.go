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
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	helloworldpb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/greeter-go/pkg/logging"
)

// intermediaryService implements helloworld.Greeter.
type intermediaryService struct {
	helloworldpb.UnimplementedGreeterServer
	logger        logr.Logger
	name          string
	greeterClient *Client
}

func NewIntermediaryService(ctx context.Context, name string, greeterClient *Client) helloworldpb.GreeterServer {
	return &intermediaryService{
		logger:        logging.FromContext(ctx),
		name:          name,
		greeterClient: greeterClient,
	}
}

func (s *intermediaryService) SayHello(ctx context.Context, request *helloworldpb.HelloRequest) (*helloworldpb.HelloReply, error) {
	s.logger.V(2).Info("Received request, forwarding to the next hop", "name", request.Name)
	intermediaryMessage, err := s.greeterClient.SayHello(ctx, request.GetName())
	if err != nil {
		logGreeterError(s.logger, err, "Greeting request failed, returning error code internal")
		st, errSt := createStatus(codes.Internal, "greeter request failed")
		if errSt != nil {
			// Should not happen
			s.logger.Error(errSt, "Could not append ErrorInfo to Status")
		}
		return nil, st.Err()
	}
	return &helloworldpb.HelloReply{Message: fmt.Sprintf("%s, via %s", intermediaryMessage, s.name)}, nil
}

func logGreeterError(logger logr.Logger, err error, message string, keysAndValues ...interface{}) {
	s, ok := status.FromError(err)
	if !ok {
		logger.Error(err, message, keysAndValues...)
		return
	}
	keysAndValues = append(keysAndValues, "code", s.Code().String(), "message", s.Message())
	for _, detail := range s.Details() {
		if message, ok := detail.(proto.Message); ok {
			messageJSONBytes, err := protojson.Marshal(message)
			if err != nil {
				logger.Error(err, "Could not marshal status detail to JSON")
			} else {
				// Log error detail if we can marshal it to JSON
				key := string(message.ProtoReflect().Descriptor().Name())
				keysAndValues = append(keysAndValues, key, string(messageJSONBytes))
			}
		}
	}
	logger.Error(err, message, keysAndValues...)
}

func createStatus(code codes.Code, message string) (*status.Status, error) {
	st := status.New(code, message)
	return st.WithDetails(&errdetails.ErrorInfo{
		Reason: code.String(),
		Domain: "greeter.example.com",
	})
}
