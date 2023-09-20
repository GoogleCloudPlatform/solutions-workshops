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

package logging

import (
	"context"

	"github.com/go-logr/logr"
)

// FromContext retrieves the logger from the context, or, is not set, returns
// a new logger.
func FromContext(ctx context.Context) logr.Logger {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		logger = NewLogger()
	}
	return logger
}

// NewContext returns a new context with the provided logr.Logger instance.
func NewContext(ctx context.Context, logger logr.Logger) context.Context {
	return logr.NewContext(ctx, logger)
}
