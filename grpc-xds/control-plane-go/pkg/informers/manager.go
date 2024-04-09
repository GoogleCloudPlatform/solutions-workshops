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
	"strings"
	"time"

	"github.com/go-logr/logr"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	discoveryinformers "k8s.io/client-go/informers/discovery/v1"
	"k8s.io/client-go/kubernetes"
	informercache "k8s.io/client-go/tools/cache"

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds"
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
	kubecontext string
	clientset   *kubernetes.Clientset
	xdsCache    *xds.SnapshotCache
}

// NewManager creates an instance that manages a collection of informers
// for one kubecontext.
func NewManager(ctx context.Context, kubecontextName string, xdsCache *xds.SnapshotCache) (*Manager, error) {
	clientset, err := NewClientSet(ctx, kubecontextName)
	if err != nil {
		return nil, err
	}
	return &Manager{
		kubecontext: kubecontextName,
		clientset:   clientset,
		xdsCache:    xdsCache,
	}, nil
}

func (m *Manager) AddEndpointSliceInformer(ctx context.Context, logger logr.Logger, config Config) error {
	logger = logger.WithValues("kubecontext", m.kubecontext, "namespace", config.Namespace)
	if config.Services == nil {
		config.Services = make([]string, 0)
	}
	labelSelector := fmt.Sprintf("%s in (%s)", discoveryv1.LabelServiceName, strings.Join(config.Services, ", "))
	logger.V(2).Info("Creating informer for EndpointSlices", "labelSelector", labelSelector)

	stop := make(chan struct{})
	go func() {
		<-ctx.Done()
		logger.V(1).Info("Stopping informer for EndpointSlices", "labelSelector", labelSelector)
		close(stop)
	}()

	factory := informers.NewSharedInformerFactory(m.clientset, 0)
	informer := factory.InformerFor(&discoveryv1.EndpointSlice{}, func(clientSet kubernetes.Interface, resyncPeriod time.Duration) informercache.SharedIndexInformer {
		indexers := informercache.Indexers{informercache.NamespaceIndex: informercache.MetaNamespaceIndexFunc}
		return discoveryinformers.NewFilteredEndpointSliceInformer(clientSet, config.Namespace, resyncPeriod, indexers, func(listOptions *metav1.ListOptions) {
			listOptions.LabelSelector = labelSelector
		})
	})

	_, err := informer.AddEventHandler(informercache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			logger := logger.WithValues("event", "add")
			logEndpointSlice(logger, obj)
			apps := getAppsForInformer(logger, informer)
			m.handleEndpointSliceEvent(ctx, logger, config.Namespace, apps)
		},
		UpdateFunc: func(_, obj interface{}) {
			logger := logger.WithValues("event", "update")
			logEndpointSlice(logger, obj)
			apps := getAppsForInformer(logger, informer)
			m.handleEndpointSliceEvent(ctx, logger, config.Namespace, apps)
		},
		DeleteFunc: func(obj interface{}) {
			logger := logger.WithValues("event", "delete")
			logEndpointSlice(logger, obj)
			apps := getAppsForInformer(logger, informer)
			m.handleEndpointSliceEvent(ctx, logger, config.Namespace, apps)
		},
	})
	if err != nil {
		return fmt.Errorf("could not add informer event handler for kubecontext=%s namespace=%s services=%+v: %w", m.kubecontext, config.Namespace, config.Services, err)
	}
	go func() {
		logger.V(2).Info("Starting informer", "services", config.Services)
		informer.Run(stop)
	}()
	return nil
}

func logEndpointSlice(logger logr.Logger, obj interface{}) {
	if logger.V(4).Enabled() {
		jsonBytes, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			logger.Error(err, "could not marshal EndpointSlice to JSON", "endpointSlice", obj)
		}
		logger.V(4).Info("Informer", "endpointSlice", string(jsonBytes))
	}
}

func (m *Manager) handleEndpointSliceEvent(ctx context.Context, logger logr.Logger, namespace string, apps []xds.GRPCApplication) {
	logger.V(2).Info("Informer resource update", "apps", apps)
	if err := m.xdsCache.UpdateResources(ctx, logger, m.kubecontext, namespace, apps); err != nil {
		// Can't propagate this error, and we probably shouldn't end the goroutine anyway.
		logger.Error(err, "Could not update the xDS resource cache with gRPC application configuration", "apps", apps)
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
		app := xds.NewGRPCApplication(namespace, k8sServiceName, port, appEndpoints)
		apps = append(apps, app)
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
