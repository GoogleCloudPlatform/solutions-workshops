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

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/informers"
)

const (
	defaultConfigDir    = "config"
	informersConfigFile = "informers.yaml"
)

var (
	errNoConfig           = errors.New("no informer configurations provided")
	errNoContext          = errors.New("no kubeconfig contexts provided")
	errNoServices         = errors.New("no services listed in informer configuration")
	errDuplicateContext   = errors.New("context name used more than once in the informer configuration")
	errDuplicateNamespace = errors.New("namespace used more than once in the informer configuration")
)

func Kubecontexts(logger logr.Logger) ([]informers.Kubecontext, error) {
	configDir, exists := os.LookupEnv("CONFIG_DIR")
	if !exists {
		configDir = defaultConfigDir
	}
	informersConfigFilePath := filepath.Join(configDir, informersConfigFile)
	logger.V(4).Info("Loading informer configuration", "filepath", informersConfigFilePath)
	yamlBytes, err := os.ReadFile(informersConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read informer configurations from file %s: %w", informersConfigFilePath, err)
	}
	var kubecontexts []informers.Kubecontext
	err = yaml.Unmarshal(yamlBytes, &kubecontexts)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal informer configuration YAML file contents [%s]: %w", yamlBytes, err)
	}
	if err := validateKubeContexts(kubecontexts); err != nil {
		return nil, fmt.Errorf("informer configuration validation failed: %w", err)
	}
	logger.V(2).Info("Informer", "configurations", kubecontexts)
	return kubecontexts, err
}

func validateKubeContexts(contexts []informers.Kubecontext) error {
	if len(contexts) == 0 {
		return errNoContext
	}
	contextNames := map[string]bool{}
	for _, context := range contexts {
		if _, exists := contextNames[context.ContextName]; exists {
			return fmt.Errorf("%w: context=%s", errDuplicateContext, context.ContextName)
		}
		if err := validateInformerConfigs(context.Informers); err != nil {
			return fmt.Errorf("invalid informer config for context=%s: %w", context.ContextName, err)
		}
		contextNames[context.ContextName] = true
	}
	return nil
}

func validateInformerConfigs(configs []informers.Config) error {
	if len(configs) == 0 {
		return errNoConfig
	}
	namespaces := map[string]bool{}
	for _, config := range configs {
		if config.Services == nil || len(config.Services) == 0 {
			return fmt.Errorf("%w: config=%+v", errNoServices, config)
		}
		if _, exists := namespaces[config.Namespace]; exists {
			return fmt.Errorf("%w: namespace=%s", errDuplicateNamespace, config.Namespace)
		}
		namespaces[config.Namespace] = true
	}
	return nil
}
