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

import com.google.common.base.Splitter;
import com.google.examples.xds.controlplane.applications.Application;
import com.google.examples.xds.controlplane.applications.ApplicationEndpoint;
import com.google.examples.xds.controlplane.applications.EndpointStatus;
import com.google.examples.xds.controlplane.xds.XdsSnapshotCache;
import io.kubernetes.client.informer.ResourceEventHandler;
import io.kubernetes.client.informer.SharedIndexInformer;
import io.kubernetes.client.informer.SharedInformer;
import io.kubernetes.client.informer.SharedInformerFactory;
import io.kubernetes.client.openapi.ApiClient;
import io.kubernetes.client.openapi.apis.DiscoveryV1Api;
import io.kubernetes.client.openapi.models.DiscoveryV1EndpointPort;
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
import java.util.Locale;
import java.util.Objects;
import java.util.Optional;
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

  /**
   * Kubernetes Service ports with one of these names will be considered health check ports
   * (case-sensitive match). This is a port naming convention invented in this sample xDS control plane
   * implementation.
   */
  private static final List<String> HEALTH_CHECK_PORT_NAMES =
      List.of("health", "healthz", "healthCheck", "healthcheck");

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
            LOG.debug(
                "informer kubecontext={} event={} endpointSlice={}",
                kubecontext,
                "add",
                endpointSlice);
            handleEndpointSliceEvent(informer, namespace);
          }

          @Override
          public void onUpdate(V1EndpointSlice previous, V1EndpointSlice endpointSlice) {
            LOG.debug(
                "informer kubecontext={} event={} endpointSlice={}",
                kubecontext,
                "update",
                endpointSlice);
            handleEndpointSliceEvent(informer, namespace);
          }

          @Override
          public void onDelete(V1EndpointSlice endpointSlice, boolean deletedFinalStateUnknown) {
            LOG.debug(
                "informer kubecontext={} event={} endpointSlice={}",
                kubecontext,
                "delete",
                endpointSlice);
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

  private void handleEndpointSliceEvent(
      @NotNull SharedIndexInformer<V1EndpointSlice> informer, @NotNull String namespace) {
    var appsForInformer =
        informer.getIndexer().list().stream()
            .filter(this::isValid)
            .map(this::toApplication)
            .collect(Collectors.toSet());
    cache.updateResources(kubecontext, namespace, appsForInformer);
  }

  @SuppressWarnings({"null", "DataFlowIssue"})
  // https://github.com/redhat-developer/vscode-java/issues/3124
  @NotNull
  private Application toApplication(@NotNull V1EndpointSlice endpointSlice) {
    List<ApplicationEndpoint> applicationEndpoints = toGrpcApplicationEndpoints(endpointSlice);
    String namespace = endpointSlice.getMetadata().getNamespace();
    String endpointSliceName = endpointSlice.getMetadata().getName();
    String k8sServiceName = endpointSlice.getMetadata().getLabels().get(LABEL_SERVICE_NAME);
    DiscoveryV1EndpointPort servingPort = findServingPort(endpointSlice);
    String servingProtocol = findProtocol(servingPort);
    // Default to using serving port for health checks, if no health check port defined on Service.
    DiscoveryV1EndpointPort healthCheckPort =
        Optional.of(findHealthCheckPort(endpointSlice)).get().orElse(servingPort);
    String healthCheckProtocol = findProtocol(healthCheckPort);
    // NOTE: Assume that the k8s ServiceAccount name matches the k8s Service name, for workload
    // identity.
    Application app =
        new Application(
            namespace,
            k8sServiceName,
            servingPort.getPort(),
            servingProtocol,
            healthCheckPort.getPort(),
            healthCheckProtocol,
            applicationEndpoints);
    LOG.debug(
        "kubecontext={} namespace={} endpointSlice={} service={} "
            + "servingPort={}({}) healthCheckPort={}({}) endpoints={}",
        kubecontext,
        namespace,
        endpointSliceName,
        k8sServiceName,
        servingPort,
        servingProtocol,
        healthCheckPort,
        healthCheckProtocol,
        applicationEndpoints);
    return app;
  }

  /**
   * Find the protocol of the provided port, in all lowercase, by considering the following:
   *
   * <ol>
   *   <li>The <a
   *       href="https://kubernetes.io/docs/concepts/services-networking/service/#application-protocol">appProtocol</a>,
   *       if set.
   *   <li>The <a
   *       href="https://kubernetes.io/docs/reference/networking/service-protocols/#protocol-support">protocol</a>,
   *       if set.
   *   <li>The default value of <code>&quot;tcp&quot;</code>.
   * </ol>
   */
  @NotNull
  private static String findProtocol(@NotNull DiscoveryV1EndpointPort port) {
    String appProtocol = port.getAppProtocol();
    if (appProtocol != null) {
      return appProtocol.toLowerCase(Locale.ROOT);
    }
    String protocol = port.getProtocol();
    if (protocol != null) {
      return protocol.toLowerCase(Locale.ROOT);
    }
    return "tcp";
  }

  /**
   * Find the first port that isn't named to identify as a health check port.
   * 
   * <p>If there is only port on the EndpointSlice, return it regardless of name.
   */
  @NotNull
  private DiscoveryV1EndpointPort findServingPort(@NotNull V1EndpointSlice endpointSlice) {
    if (endpointSlice.getPorts() == null || endpointSlice.getPorts().isEmpty()) {
      throw new IllegalArgumentException("endpointSlice ports must be non-null and non-empty");
    }
    return endpointSlice.getPorts().stream()
        .filter(
            port ->
                port.getPort() != null
                    && (port.getName() == null
                        || !HEALTH_CHECK_PORT_NAMES.contains(port.getName())))
        .findFirst()
        .orElse(
            endpointSlice
                .getPorts()
                .get(0)); // if there is only one port, use it regardless of name
  }

  /**
   * Find the first port that is named to identify as a health check port.
   * 
   * <p>Returns an empty Optional if no ports are named to identify as health check ports.
   */
  @NotNull
  private Optional<DiscoveryV1EndpointPort> findHealthCheckPort(
      @NotNull V1EndpointSlice endpointSlice) {
    if (endpointSlice.getPorts() == null || endpointSlice.getPorts().isEmpty()) {
      throw new IllegalArgumentException("endpointSlice ports must be non-null and non-empty");
    }
    return endpointSlice.getPorts().stream()
        .filter(
            port ->
                port.getPort() != null
                    && port.getName() != null
                    && HEALTH_CHECK_PORT_NAMES.contains(port.getName()))
        .findFirst();
  }

  @NotNull
  private List<ApplicationEndpoint> toGrpcApplicationEndpoints(
      @NotNull V1EndpointSlice endpointSlice) {
    return endpointSlice.getEndpoints().stream()
        .map(
            // https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#topology
            endpoint ->
                new ApplicationEndpoint(
                    Objects.requireNonNullElse(endpoint.getNodeName(), ""),
                    Objects.requireNonNullElse(endpoint.getZone(), ""),
                    endpoint.getAddresses(),
                    EndpointStatus.fromConditions(endpoint.getConditions())))
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
      LOG.error("Skipping EndpointSlice due to missing servingPort: {}", endpointSlice);
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
    List<String> filePaths = Splitter.onPattern(File.pathSeparator).splitToList(kubeConfigEnv);
    String kubeConfigPath = filePaths.get(0);
    if (filePaths.size() > 1) {
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
        LOG.info("Using kubeconfig context={}", kubecontext);
        config.setContext(kubecontext);
      } else {
        LOG.info("Using current kubeconfig context={}", config.getCurrentContext());
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
