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
	"fmt"

	"github.com/envoyproxy/go-control-plane/pkg/log"
	"github.com/go-logr/logr"
)

const (
	cacheLoggerCallDepth = 1
)

// xdsCacheLogger implements github.com/envoyproxy/go-control-plane/pkg/log.Logger
// See: https://github.com/envoyproxy/go-control-plane/blob/v0.11.1/pkg/log/log.go
type xdsCacheLogger struct {
	logr.Logger
}

func SnapshotCacheLogger(ctx context.Context) log.Logger {
	return &xdsCacheLogger{
		FromContext(ctx).WithCallDepth(cacheLoggerCallDepth),
	}
}

func (l xdsCacheLogger) Debugf(format string, args ...interface{}) {
	l.V(4).Info(fmt.Sprintf(format, args...))
}

func (l xdsCacheLogger) Infof(format string, args ...interface{}) {
	l.V(2).Info(fmt.Sprintf(format, args...))
}

func (l xdsCacheLogger) Warnf(format string, args ...interface{}) {
	l.V(1).Info(fmt.Sprintf(format, args...))
}

func (l xdsCacheLogger) Errorf(format string, args ...interface{}) {
	l.Error(nil, fmt.Sprintf(format, args...))
}
