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

	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/applications"
	"github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go/pkg/xds"
)

var (
	errMissingLabel           = errors.New("missing service label")
	errMissingMetadata        = errors.New("missing metadata")
	errNilEndpointSlice       = errors.New("nil EndpointSlice")
	errNoPortsInEndpointSlice = errors.New("no ports in EndpointSlice")
	errUnexpectedType         = errors.New("unexpected type")
	// Kubernetes Service ports with one of these names will be considered health check ports (case-sensitive match). This is a port naming convention invented in this sample xDS control plane implementation.
	healthCheckPortNames = map[string]bool{
		"health":      true,
		"healthz":     true,
		"healthCheck": true,
		"healthcheck": true,
	}
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

func (m *Manager) handleEndpointSliceEvent(ctx context.Context, logger logr.Logger, namespace string, apps []applications.Application) {
	logger.V(2).Info("Informer resource update", "apps", apps)
	if err := m.xdsCache.UpdateResources(ctx, logger, m.kubecontext, namespace, apps); err != nil {
		// Can't propagate this error, and we probably shouldn't end the goroutine anyway.
		logger.Error(err, "Could not update the xDS resource cache with gRPC application configuration", "apps", apps)
	}
}

func getAppsForInformer(logger logr.Logger, informer informercache.SharedIndexInformer) []applications.Application {
	var apps []applications.Application
	for _, eps := range informer.GetIndexer().List() {
		endpointSlice, err := validateEndpointSlice(eps)
		if err != nil {
			logger.Error(err, "Skipping EndpointSlice")
			continue
		}
		k8sServiceName := endpointSlice.GetObjectMeta().GetLabels()[discoveryv1.LabelServiceName]
		namespace := endpointSlice.GetObjectMeta().GetNamespace()
		servingPort := findServingPort(endpointSlice)
		healthCheckPort, exists := findHealthCheckPort(endpointSlice)
		if !exists {
			// Default to using the serving port for health checks.
			healthCheckPort = servingPort
		}
		servingProtocol := findProtocol(servingPort)
		healthCheckProtocol := findProtocol(healthCheckPort)
		appEndpoints := getApplicationEndpoints(endpointSlice)
		app := applications.NewApplication(namespace, k8sServiceName, uint32(*servingPort.Port), servingProtocol, uint32(*healthCheckPort.Port), healthCheckProtocol, appEndpoints)
		apps = append(apps, app)
	}
	return apps
}

// getProtocol returns the protocol of the provided port, in all lowercase, by considering the following:
//
// 1.  The [appProtocol](https://kubernetes.io/docs/concepts/services-networking/service/#application-protocol), if set.
// 2.  The [protocol](https://kubernetes.io/docs/reference/networking/service-protocols/#protocol-support), if set.
// 3.  The default value of `tcp`.
func findProtocol(port discoveryv1.EndpointPort) string {
	if port.AppProtocol != nil {
		return strings.ToLower(*port.AppProtocol)
	}
	if port.Protocol != nil {
		return strings.ToLower(string(*port.Protocol))
	}
	return "tcp"
}

// findServingPort returns the first port that isn't named to identify as a health check port.
// If there is only port on the EndpointSlice, return it regardless of name.
func findServingPort(endpointSlice *discoveryv1.EndpointSlice) discoveryv1.EndpointPort {
	for _, endpointPort := range endpointSlice.Ports {
		if endpointPort.Port != nil && (endpointPort.Name == nil || !healthCheckPortNames[*endpointPort.Name]) {
			return endpointPort
		}
	}
	// If all ports are named as health check ports, use the first one, regardless of name.
	return endpointSlice.Ports[0]
}

// findHealthCheckPort returns the first port that is named to identify as a health check port.
// Returns `false` as the second return value if no ports are named to identify as health check ports.
func findHealthCheckPort(endpointSlice *discoveryv1.EndpointSlice) (discoveryv1.EndpointPort, bool) {
	for _, endpointPort := range endpointSlice.Ports {
		if endpointPort.Name != nil && healthCheckPortNames[*endpointPort.Name] {
			return endpointPort, true
		}
	}
	return discoveryv1.EndpointPort{}, false
}

// getApplicationEndpoints returns the endpoints as `GRPCApplicationEndpoints`.
func getApplicationEndpoints(endpointSlice *discoveryv1.EndpointSlice) []applications.ApplicationEndpoints {
	var appEndpoints []applications.ApplicationEndpoints
	for _, endpoint := range endpointSlice.Endpoints {
		if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
			var k8sNode, zone string
			if endpoint.NodeName != nil {
				k8sNode = *endpoint.NodeName
			}
			if endpoint.Zone != nil {
				zone = *endpoint.Zone
			}
			appEndpoints = append(appEndpoints, applications.NewApplicationEndpoints(k8sNode, zone, endpoint.Addresses, applications.EndpointStatusFromConditions(endpoint.Conditions)))
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
	nonNilPortNumberExists := false
	for _, endpointPort := range endpointSlice.Ports {
		if endpointPort.Port != nil {
			nonNilPortNumberExists = true
		}
	}
	if !nonNilPortNumberExists {
		return nil, fmt.Errorf("%w: %+v", errNoPortsInEndpointSlice, endpointSlice)
	}
	return endpointSlice, nil
}
