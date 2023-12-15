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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	discoveryinformers "k8s.io/client-go/informers/discovery/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	informercache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/logging"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go/pkg/xds"
)

var (
	errMissingLabel           = errors.New("missing service label")
	errMissingMetadata        = errors.New("missing metadata")
	errNilEndpointSlice       = errors.New("nil EndpointSlice")
	errNoPortsInEndpointSlice = errors.New("no ports in EndpointSlice")
	errUnexpectedType         = errors.New("unexpected type")
)

// Manager manages a collection of informers.
type Manager struct {
	clientset *kubernetes.Clientset
	logger    logr.Logger
	xdsCache  *xds.SnapshotCache
	informers []informercache.SharedIndexInformer
}

// NewManager creates an instance that manages a collection of informers.
func NewManager(ctx context.Context, xdsCache *xds.SnapshotCache) (*Manager, error) {
	logger := logging.FromContext(ctx)
	config, err := clientConfig(logger)
	if err != nil {
		return nil, fmt.Errorf("could not create kubernetes config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create kubernetes clientset for config=%+v: %w", config, err)
	}
	return &Manager{
		clientset: clientset,
		logger:    logger,
		xdsCache:  xdsCache,
	}, nil
}

// clientConfig uses in-cluster config if the directory
// `/var/run/secrets/kubernetes.io` exists and the user has the
// appropriate permissions. This directory exists in Kubernetes Pods, unless
// `automountServiceAccountToken` is `false`, and `true` is the default value.
// [Reference]: https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/
//
// If the directory does not exist, configuration is read from a kubeconfig file.
func clientConfig(logger logr.Logger) (*rest.Config, error) {
	if _, err := os.Stat("/var/run/secrets/kubernetes.io"); os.IsNotExist(err) {
		logger.V(2).Info("using kubeconfig file", "kubeconfig", kubeconfig)
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	logger.V(2).Info("using in-cluster config")
	return rest.InClusterConfig()
}

func (m *Manager) AddEndpointSliceInformer(ctx context.Context, config Config) error {
	if config.Services == nil {
		config.Services = make([]string, 0)
	}
	labelSelector := fmt.Sprintf("%s in (%s)", discoveryv1.LabelServiceName, strings.Join(config.Services, ", "))
	m.logger.V(2).Info("Creating informer for EndpointSlices", "namespace", config.Namespace, "labelSelector", labelSelector)

	stop := make(chan struct{})
	go func() {
		<-ctx.Done()
		m.logger.V(1).Info("Stopping informer for EndpointSlices", "namespace", config.Namespace, "labelSelector", labelSelector)
		close(stop)
	}()

	factory := informers.NewSharedInformerFactory(m.clientset, 0)
	informer := factory.InformerFor(&discoveryv1.EndpointSlice{}, func(clientSet kubernetes.Interface, resyncPeriod time.Duration) informercache.SharedIndexInformer {
		indexers := informercache.Indexers{informercache.NamespaceIndex: informercache.MetaNamespaceIndexFunc}
		return discoveryinformers.NewFilteredEndpointSliceInformer(clientSet, config.Namespace, resyncPeriod, indexers, func(listOptions *metav1.ListOptions) {
			listOptions.LabelSelector = labelSelector
		})
	})
	m.informers = append(m.informers, informer)

	_, err := informer.AddEventHandler(informercache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			m.handleEndpointSliceEvent(ctx, "add", obj)
		},
		UpdateFunc: func(oldObj, obj interface{}) {
			m.handleEndpointSliceEvent(ctx, "update", obj)
		},
		DeleteFunc: func(obj interface{}) {
			m.handleEndpointSliceEvent(ctx, "delete", obj)
		},
	})
	if err != nil {
		return fmt.Errorf("could not add informer event handler for namespace=%s services=%+v: %w", config.Namespace, config.Services, err)
	}

	go func() {
		m.logger.V(2).Info("Starting informer", "config", config)
		informer.Run(stop)
	}()
	return nil
}

func (m *Manager) handleEndpointSliceEvent(ctx context.Context, event string, obj interface{}) {
	if m.logger.V(4).Enabled() {
		jsonBytes, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			m.logger.Error(err, "could not marshal EndpointSlice to JSON", "endpointSlice", obj)
		}
		m.logger.V(4).Info(event, "endpointSlice", string(jsonBytes))
	}
	var apps []xds.GRPCApplication
	for _, informer := range m.informers {
		appsForInformer := getAppsForInformer(m.logger, informer)
		apps = append(apps, appsForInformer...)
	}
	if err := m.xdsCache.UpdateResources(ctx, apps); err != nil {
		// Can't propagate this error, and we probably shouldn't end the goroutine anyway.
		m.logger.Error(err, "Could not update the xDS resource cache with new gRPC application configuration", "apps", apps)
	}
}

func getAppsForInformer(logger logr.Logger, informer informercache.SharedIndexInformer) []xds.GRPCApplication {
	var apps []xds.GRPCApplication
	for _, eps := range informer.GetIndexer().List() {
		endpointSlice, err := validateEndpointSlice(eps)
		if err != nil {
			logger.Error(err, "Skipping EndpointSlice")
			continue
		}
		k8sServiceName := endpointSlice.GetObjectMeta().GetLabels()[discoveryv1.LabelServiceName]
		namespace := endpointSlice.GetObjectMeta().GetNamespace()
		// TODO: Handle more than one port?
		port := uint32(*endpointSlice.Ports[0].Port)
		appEndpoints := getReadyApplicationEndpoints(endpointSlice)
		logger.V(4).Info("Ready endpoints for service", "name", k8sServiceName, "namespace", namespace, "appEndpoints", appEndpoints)
		apps = append(apps, xds.NewGRPCApplication(namespace, k8sServiceName, port, appEndpoints))
	}
	return apps
}

// getReadyApplicationEndpoints filters the k8s endpoints in the EndpointSlice to only include `Ready` endpoints.
// The function returns the endpoints as `GRPCApplicationEndpoints`.
func getReadyApplicationEndpoints(endpointSlice *discoveryv1.EndpointSlice) []xds.GRPCApplicationEndpoints {
	var appEndpoints []xds.GRPCApplicationEndpoints
	for _, endpoint := range endpointSlice.Endpoints {
		if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
			var k8sNode, zone string
			if endpoint.NodeName != nil {
				k8sNode = *endpoint.NodeName
			}
			if endpoint.Zone != nil {
				zone = *endpoint.Zone
			}
			appEndpoints = append(appEndpoints, xds.NewGRPCApplicationEndpoints(k8sNode, zone, endpoint.Addresses))
		}
	}
	return appEndpoints
}

// validateEndpointSlice ensures that the EndpointSlice contains the fields
// required to turn it into a `xds.GRPCApplication` instance.
func validateEndpointSlice(eps interface{}) (*discoveryv1.EndpointSlice, error) {
	if eps == nil {
		return nil, errNilEndpointSlice
	}
	endpointSlice, ok := eps.(*discoveryv1.EndpointSlice)
	if !ok {
		return nil, fmt.Errorf("%w: expected *discoveryv1.EndpointSlice, got %T", errUnexpectedType, eps)
	}
	if endpointSlice.GetObjectMeta().GetName() == "" ||
		endpointSlice.GetObjectMeta().GetNamespace() == "" {
		return nil, fmt.Errorf("%w from EndpointSlice %+v", errMissingMetadata, endpointSlice)
	}
	if endpointSlice.GetObjectMeta().GetLabels() == nil ||
		len(endpointSlice.GetObjectMeta().GetLabels()[discoveryv1.LabelServiceName]) == 0 {
		return nil, fmt.Errorf("%w from EndpointSlice %+v", errMissingLabel, endpointSlice)
	}
	if len(endpointSlice.Ports) == 0 {
		return nil, fmt.Errorf("%w: %+v", errNoPortsInEndpointSlice, endpointSlice)
	}
	return endpointSlice, nil
}
