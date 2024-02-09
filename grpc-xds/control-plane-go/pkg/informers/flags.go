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
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	configPathFlagUsage = "absolute path to the kubeconfig file(s), colon-separated if multiple files"

	// Do not change the values below from their recommended values in clientcmd:.
	configPathEnvVar = clientcmd.RecommendedConfigPathEnvVar
	configPathFlag   = clientcmd.RecommendedConfigPathFlag
	configHomeDir    = clientcmd.RecommendedHomeDir
	configFileName   = clientcmd.RecommendedFileName
)

var (
	kubeconfig  string
	commandLine flag.FlagSet
)

func init() {
	usagePrefix := "(optional) "
	if _, err := os.Stat("/var/run/secrets/kubernetes.io"); os.IsNotExist(err) {
		// Not running in a Kubernetes Pod, or the Kubernetes Service Account is not mounted,
		// or the user does not have the appropriate permissions to read the service account token.
		// [Reference]: https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/
		// In-cluster config is impossible in this case, and a kubeconfig file is required.
		usagePrefix = ""
	}
	usage := usagePrefix + configPathFlagUsage
	if kubeconfigEnvVarValue, exists := os.LookupEnv(configPathEnvVar); exists {
		commandLine.StringVar(&kubeconfig, configPathFlag, kubeconfigEnvVarValue, usage)
	} else if home := homedir.HomeDir(); home != "" {
		commandLine.StringVar(&kubeconfig, configPathFlag, filepath.Join(home, configHomeDir, configFileName), usage)
	} else {
		commandLine.StringVar(&kubeconfig, configPathFlag, "", usage)
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
