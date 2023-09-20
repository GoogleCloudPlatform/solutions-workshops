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
	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
)

// Ref: https://github.com/kubernetes/community/blob/1a09f121536ddb84c1429c88fbb3978d6c5e2dd0/contributors/devel/sig-instrumentation/logging.md

func NewLogger() logr.Logger {
	logger := klog.NewKlogr()
	logger.WithCallDepth(2).V(1).Info("Creating new logger")
	return logger
}
