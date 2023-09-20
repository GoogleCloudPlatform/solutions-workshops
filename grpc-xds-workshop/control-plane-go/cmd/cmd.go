// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"flag"
	"fmt"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/config"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/logging"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/server"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/signals"
)

func Run(ctx context.Context, flagset *flag.FlagSet, args []string) error {
	ctx = signals.SetupSignalHandler(ctx)
	logging.InitFlags(flagset)
	if err := flagset.Parse(args); err != nil {
		return fmt.Errorf("could not parse command line flags args=%+v: %w", args, err)
	}
	logger := logging.NewLogger()
	logging.SetGRPCLogger(logger)
	ctx = logging.NewContext(ctx, logger)
	port, err := config.Port()
	if err != nil {
		return fmt.Errorf("could not configure management server listening port: %w", err)
	}
	informerConfigs, err := config.Informers(ctx)
	if err != nil {
		return fmt.Errorf("could not initialize informer configuration: %w", err)
	}
	return server.Run(ctx, port, informerConfigs)
}
