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
	"fmt"

	"github.com/go-logr/logr"
	"google.golang.org/grpc/grpclog"
)

const logrWrapperDepth = 2

// logrWrapper implements `grpclog.LoggerV2`.
type logrWrapper struct {
	logger logr.Logger
}

var (
	_ grpclog.LoggerV2      = &logrWrapper{}
	_ grpclog.DepthLoggerV2 = &logrWrapper{}
)

func SetGRPCLogger(logger logr.Logger) {
	grpclog.SetLoggerV2(&logrWrapper{
		logger: logger.WithCallDepth(logrWrapperDepth),
	})
}

func (w *logrWrapper) V(l int) bool {
	return w.logger.V(l).Enabled()
}

func (w *logrWrapper) Info(args ...any) {
	w.logger.V(2).Info(fmt.Sprint(args...))
}

func (w *logrWrapper) Infoln(args ...any) {
	w.logger.V(2).Info(fmt.Sprint(args...))
}

func (w *logrWrapper) Infof(format string, args ...any) {
	w.logger.V(2).Info(fmt.Sprintf(format, args...))
}

func (w *logrWrapper) InfoDepth(depth int, args ...any) {
	w.logger.WithCallDepth(depth).V(2).Info(fmt.Sprint(args...))
}

func (w *logrWrapper) Warning(args ...any) {
	w.logger.V(1).Info(fmt.Sprint(args...))
}

func (w *logrWrapper) Warningln(args ...any) {
	w.logger.V(1).Info(fmt.Sprint(args...))
}

func (w *logrWrapper) Warningf(format string, args ...any) {
	w.logger.V(1).Info(fmt.Sprintf(format, args...))
}

func (w *logrWrapper) WarningDepth(depth int, args ...any) {
	w.logger.WithCallDepth(depth).V(0).Info(fmt.Sprint(args...))
}

func (w *logrWrapper) Error(args ...any) {
	w.logger.Error(nil, fmt.Sprint(args...))
}

func (w *logrWrapper) Errorln(args ...any) {
	w.logger.Error(nil, fmt.Sprint(args...))
}

func (w *logrWrapper) Errorf(format string, args ...any) {
	w.logger.Error(nil, fmt.Sprintf(format, args...))
}

func (w *logrWrapper) ErrorDepth(depth int, args ...any) {
	w.logger.WithCallDepth(depth).Error(nil, fmt.Sprint(args...))
}

func (w *logrWrapper) Fatal(args ...any) {
	w.logger.Error(nil, fmt.Sprint(args...))
}

func (w *logrWrapper) Fatalln(args ...any) {
	w.logger.Error(nil, fmt.Sprint(args...))
}

func (w *logrWrapper) Fatalf(format string, args ...any) {
	w.logger.Error(nil, fmt.Sprintf(format, args...))
}

func (w *logrWrapper) FatalDepth(depth int, args ...any) {
	w.logger.WithCallDepth(depth).Error(nil, fmt.Sprint(args...))
}
