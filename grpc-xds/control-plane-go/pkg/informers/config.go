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

package informers

// Config represents a collection of Kubernetes services in a namespace.
type Config struct {
	Namespace string   `yaml:"namespace"`
	Services  []string `yaml:"services"`
}

// Kubecontext represents a kubeconfig context,
// containing a list of `informer.Config`s.
type Kubecontext struct {
	Context   string   `yaml:"context"`
	Informers []Config `yaml:"informers"`
}
