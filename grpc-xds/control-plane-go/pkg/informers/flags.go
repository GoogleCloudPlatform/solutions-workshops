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

import (
	"flag"
	"path/filepath"

	"k8s.io/client-go/util/homedir"
)

var (
	kubeconfig  string
	commandLine flag.FlagSet
)

func init() {
	if home := homedir.HomeDir(); home != "" {
		commandLine.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		commandLine.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
}

// InitFlags initializes flags for the Kubernetes client.
func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}
	commandLine.VisitAll(func(f *flag.Flag) {
		flagset.Var(f.Value, f.Name, f.Usage)
	})
}
