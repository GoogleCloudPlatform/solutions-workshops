// Copyright 2024 Google LLC
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
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/logging"
)

func NewClientSet(ctx context.Context, kubecontextName string) (*kubernetes.Clientset, error) {
	logger := logging.FromContext(ctx)
	config, err := clientConfig(logger, kubecontextName)
	if err != nil {
		return nil, fmt.Errorf("could not create Kubernetes config for context=%s: %w", kubecontextName, err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create Kubernetes clientset for context=%s and config=%+v: %w", kubecontextName, config, err)
	}
	return clientset, nil
}

// clientConfig uses in-cluster config if the values of the kubeconfig flag
// and KUBECONFIG environment variable are empty. Otherwise, the specified
// kubeconfig files are parsed, and the provided kubecontextName is selected
// as the current context.
func clientConfig(logger logr.Logger, kubecontextName string) (*rest.Config, error) {
	if kubeconfig == "" {
		logger.V(2).Info("using in-cluster config")
		if kubecontextName != "" {
			logger.Info("ignoring provided kubeconfig context name, as we are using in-cluster config", "context", kubecontextName)
		}
		return rest.InClusterConfig()
	}
	logger.V(2).Info("using kubeconfig file(s)", "kubeconfig", kubeconfig, "context", kubecontextName)
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{
			CurrentContext: kubecontextName,
		}).ClientConfig()
}
