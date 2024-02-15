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

package com.google.examples.xds.controlplane.informers;

import com.google.examples.xds.controlplane.xds.GrpcApplication;
import com.google.examples.xds.controlplane.xds.GrpcApplicationEndpoint;
import com.google.examples.xds.controlplane.xds.XdsSnapshotCache;
import io.kubernetes.client.informer.ResourceEventHandler;
import io.kubernetes.client.informer.SharedIndexInformer;
import io.kubernetes.client.informer.SharedInformer;
import io.kubernetes.client.informer.SharedInformerFactory;
import io.kubernetes.client.openapi.ApiClient;
import io.kubernetes.client.openapi.apis.DiscoveryV1Api;
import io.kubernetes.client.openapi.models.V1EndpointSlice;
import io.kubernetes.client.openapi.models.V1EndpointSliceList;
import io.kubernetes.client.util.Config;
import io.kubernetes.client.util.KubeConfig;
import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Objects;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import okhttp3.OkHttpClient;
import okhttp3.logging.HttpLoggingInterceptor;
import okhttp3.logging.HttpLoggingInterceptor.Level;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Manages a collection of informers. Each informer watches Kubernetes EndpointSlices in a
 * namespace, and updates the xDS snapshot cache on changes.
 *
 * <p><a
 * href="https://github.com/kubernetes-client/java/blob/8fd1cf4b4f91dd8591c5d2cb254a99efec89f004/examples/examples-release-18/src/main/java/io/kubernetes/client/examples/InformerExample.java">example</a>
 */
public class InformerManager<T> {
  private static final Logger LOG = LoggerFactory.getLogger(InformerManager.class);

  /** Label used to filter which EndpointSlices to watch. */
  private static final String LABEL_SERVICE_NAME = "kubernetes.io/service-name";

  /** Kubernetes API client. */
  private final ApiClient client;

  /** EndpointSlices belong to the <code>discovery.k8s.io/v1</code> API group/version. */
  private final DiscoveryV1Api discoveryV1Api;

  private final String kubecontext;

  /** This class updates the snapshot cache when there are changes to the watched EndpointSlices. */
  private final @NotNull XdsSnapshotCache<T> cache;

  private final List<SharedIndexInformer<V1EndpointSlice>> informers;

  /** Creates the Kubernetes client informer gubbins. */
  public InformerManager(@NotNull String kubecontext, @NotNull XdsSnapshotCache<T> cache) {
    this.kubecontext = kubecontext;
    this.cache = cache;
    this.client = createK8sApiClient(kubecontext);
    this.discoveryV1Api = new DiscoveryV1Api(this.client);
    this.informers = new ArrayList<>();
  }

  /**
   * Creates an informer on EndpointSlices, either in a namespace, or cluster-wide.
   *
   * @param namespace the Kubernetes Namespace to watch, or an empty string for a cluster-wide watch
   * @param services Kubernetes Services that owns the EndpointSlices to watch
   */
  public void addEndpointSliceInformer(
      @NotNull String namespace, @NotNull Collection<String> services) {
    var labelSelector = LABEL_SERVICE_NAME + " in (" + String.join(", ", services) + ")";
    LOG.info(
        "Creating informer for EndpointSlices in kubecontext={} namespace={} "
            + "with labelSelector=[{}]",
        kubecontext,
        namespace,
        labelSelector);

    SharedIndexInformer<V1EndpointSlice> informer =
        new SharedInformerFactory(client)
            .sharedIndexInformerFor(
                params ->
                    discoveryV1Api
                        .listNamespacedEndpointSlice(namespace)
                        .allowWatchBookmarks(Boolean.TRUE)
                        .labelSelector(labelSelector)
                        .resourceVersion(params.resourceVersion)
                        .timeoutSeconds(params.timeoutSeconds)
                        .watch(params.watch)
                        .buildCall(null),
                V1EndpointSlice.class,
                V1EndpointSliceList.class);

    this.informers.add(informer);

    informer.addEventHandler(
        new ResourceEventHandler<>() {
          @Override
          public void onAdd(V1EndpointSlice endpointSlice) {
            LOG.debug("informer kubecontext={} event={} endpointSlice={}", kubecontext, "add", endpointSlice);
            handleEndpointSliceEvent(informer, namespace);
          }

          @Override
          public void onUpdate(V1EndpointSlice previous, V1EndpointSlice endpointSlice) {
            LOG.debug("informer kubecontext={} event={} endpointSlice={}", kubecontext, "update", endpointSlice);
            handleEndpointSliceEvent(informer, namespace);
          }

          @Override
          public void onDelete(V1EndpointSlice endpointSlice, boolean deletedFinalStateUnknown) {
            LOG.debug("informer kubecontext={} event={} endpointSlice={}", kubecontext, "delete", endpointSlice);
            handleEndpointSliceEvent(informer, namespace);
          }
        });
  }

  /** Start the informers. */
  public void start() {
    informers.forEach(SharedInformer::run);
  }

  /** Stop the informers. */
  public void stop() {
    informers.forEach(SharedInformer::stop);
  }

  private void handleEndpointSliceEvent(@NotNull SharedIndexInformer<V1EndpointSlice> informer, @NotNull String namespace) {
    var appsForInformer = informer.getIndexer().list().stream().filter(this::isValid).map(this::toGrpcApplication).collect(Collectors.toSet());
    cache.updateResources(kubecontext, namespace, appsForInformer);
  }

  @SuppressWarnings({"null"})
  // https://github.com/redhat-developer/vscode-java/issues/3124
  @NotNull
  private GrpcApplication toGrpcApplication(@NotNull V1EndpointSlice endpointSlice) {
    List<GrpcApplicationEndpoint> applicationEndpoints = toGrpcApplicationEndpoints(endpointSlice);
    String namespace = endpointSlice.getMetadata().getNamespace();
    String endpointSliceName = endpointSlice.getMetadata().getName();
    String k8sServiceName = endpointSlice.getMetadata().getLabels().get(LABEL_SERVICE_NAME);
    // TODO: Handle more than one port?
    int port = endpointSlice.getPorts().get(0).getPort();
    // Assume that the k8s ServiceAccount name matches the k8s Service name!
    GrpcApplication app =
        new GrpcApplication(namespace, k8sServiceName, port, applicationEndpoints);
    LOG.debug(
        "kubecontext={} namespace={} endpointSlice={} service={} port={} endpoints={}",
        kubecontext,
        namespace,
        endpointSliceName,
        k8sServiceName,
        port,
        applicationEndpoints);
    return app;
  }

  @NotNull
  private List<GrpcApplicationEndpoint> toGrpcApplicationEndpoints(
      @NotNull V1EndpointSlice endpointSlice) {
    return endpointSlice.getEndpoints().stream()
        .filter(
            // https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#ready
            endpoint ->
                endpoint.getConditions() != null
                    && Boolean.TRUE.equals(endpoint.getConditions().getReady()))
        .map(
            // https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#topology
            endpoint ->
                new GrpcApplicationEndpoint(
                    Objects.requireNonNullElse(endpoint.getNodeName(), ""),
                    Objects.requireNonNullElse(endpoint.getZone(), ""),
                    endpoint.getAddresses()))
        .toList();
  }

  @SuppressWarnings("null") // https://github.com/redhat-developer/vscode-java/issues/3124
  private boolean isValid(V1EndpointSlice endpointSlice) {
    if (endpointSlice == null) {
      LOG.error("Skipping null EndPointSlice");
      return false;
    }
    if (endpointSlice.getMetadata() == null
        || endpointSlice.getMetadata().getName() == null
        || endpointSlice.getMetadata().getNamespace() == null) {
      LOG.error("Skipping EndpointSlice due to missing metadata: {}", endpointSlice);
      return false;
    }
    if (endpointSlice.getMetadata() == null
        || endpointSlice.getMetadata().getLabels() == null
        || endpointSlice.getMetadata().getLabels().get(LABEL_SERVICE_NAME) == null) {
      LOG.error("Skipping EndpointSlice due to missing labels: {}", endpointSlice);
      return false;
    }
    if (endpointSlice.getPorts() == null || endpointSlice.getPorts().isEmpty()) {
      LOG.error("Skipping EndpointSlice due to missing port: {}", endpointSlice);
      return false;
    }
    return true;
  }

  private ApiClient createK8sApiClient(String kubecontext) {
    ApiClient client = createApiClient(kubecontext);
    OkHttpClient httpClient =
        client
            .getHttpClient()
            .newBuilder()
            // Level.HEADERS and Level.BODY will include sensitive information, such as the
            // authorization bearer token. See `logging.properties` for a filter that can mask
            // this token.
            .addInterceptor(new HttpLoggingInterceptor().setLevel(Level.BASIC))
            .readTimeout(0, TimeUnit.SECONDS)
            .build();
    client.setHttpClient(httpClient);
    return client;
  }

  private ApiClient createApiClient(String kubecontext) {
    String kubeConfigEnv = System.getenv(Config.ENV_KUBECONFIG);
    if (kubeConfigEnv == null || kubeConfigEnv.isBlank()) {
      LOG.info("Using in-cluster Kubernetes client configuration");
      try {
        return Config.defaultClient();
      } catch (IOException e) {
        throw new InformerException(
            "Could not create Kubernetes client using in-cluster config", e);
      }
    }
    String[] filePaths = kubeConfigEnv.split(File.pathSeparator);
    String kubeConfigPath = filePaths[0];
    if (filePaths.length > 1) {
      LOG.warn(
          "Found multiple kubeconfig files, using first file only, KUBECONFIG={}", kubeConfigEnv);
    }
    LOG.info("Using Kubernetes client configuration from file {}", kubeConfigPath);
    try (BufferedReader bufferedReader =
        new BufferedReader(
            new InputStreamReader(new FileInputStream(kubeConfigPath), StandardCharsets.UTF_8))) {
      KubeConfig config = KubeConfig.loadKubeConfig(bufferedReader);
      config.setFile(new File(kubeConfigPath));
      if (kubecontext != null && !kubecontext.isBlank()) {
        LOG.info("Using kubeconfig context {}", kubecontext);
        config.setContext(kubecontext);
      }
      return Config.fromConfig(config);
    } catch (IOException e) {
      throw new InformerException(
          "Could not create Kubernetes client using kubeconfig="
              + kubeConfigPath
              + " and kubecontext="
              + kubecontext);
    }
  }
}
