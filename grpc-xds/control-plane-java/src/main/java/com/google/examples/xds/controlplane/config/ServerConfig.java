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

package com.google.examples.xds.controlplane.config;

import com.google.examples.xds.controlplane.informers.InformerConfig;
import com.google.examples.xds.controlplane.informers.Kubecontext;
import com.google.examples.xds.controlplane.xds.XdsFeatures;
import java.io.IOException;
import java.net.InetAddress;
import java.net.UnknownHostException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.regex.Pattern;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.snakeyaml.engine.v2.api.Load;
import org.snakeyaml.engine.v2.api.LoadSettings;

/**
 * Reads control plane management server configuration from the environment.
 *
 * <p>For simplified unit testing of the server, create a subclass and override methods.
 */
public class ServerConfig {

  private static final Logger LOG = LoggerFactory.getLogger(ServerConfig.class);

  /**
   * Default management server servingPort. Override with the <code>PORT</code> environment
   * variable.
   */
  private static final int DEFAULT_SERVING_PORT = 50051;

  /**
   * Default health checking servingPort. Override with the <code>HEALTH_PORT</code> environment
   * variable.
   */
  private static final int DEFAULT_HEALTH_PORT = 50052;

  /** The file that lists the K8s Services whose EndpointSlices should be watched. */
  private static final String INFORMERS_CONFIG_FILE = "config/informers.yaml";

  /** The file that contains xDS feature toggles. */
  private static final String XDS_FEATURES_CONFIG_FILE = "config/xds_features.yaml";

  /**
   * The file that contains the {@code app.kubernetes.io/name} pod label, mounted using the k8s
   * downward API.
   */
  private static final String APP_NAME_LABEL_FILEPATH_DOWNWARD_API = "/etc/podinfo/label-app-name";

  /** The file that contains the pod namespace, mounted using the k8s downward API. */
  private static final String NAMESPACE_FILEPATH_DOWNWARD_API = "/etc/podinfo/namespace";

  /**
   * The file that contains the pod namespace if the Kubernetes service account token is mounted.
   */
  private static final String NAMESPACE_FILEPATH_SERVICE_ACCOUNT =
      "/var/run/secrets/kubernetes.io/serviceaccount/namespace";

  /** Reference to the Kubernetes API server. */
  private static final String KUBERNETES_SERVICE = "kubernetes.default.svc";

  /**
   * Returns the servingPort number that the management server should listen on for xDS requests.
   */
  public int servingPort() {
    String portEnv = System.getenv("PORT");
    if (portEnv != null && !portEnv.isBlank()) {
      try {
        return Integer.parseInt(portEnv);
      } catch (NumberFormatException e) {
        throw new RuntimeException("Invalid value of PORT environment variable: " + portEnv, e);
      }
    }
    return DEFAULT_SERVING_PORT;
  }

  /**
   * Returns the servingPort number that the management server should listen on for health requests.
   */
  public int healthPort() {
    String portEnv = System.getenv("HEALTH_PORT");
    if (portEnv != null && !portEnv.isBlank()) {
      try {
        return Integer.parseInt(portEnv);
      } catch (NumberFormatException e) {
        throw new RuntimeException(
            "Invalid value of HEALTH_PORT environment variable: " + portEnv, e);
      }
    }
    return DEFAULT_HEALTH_PORT;
  }

  /**
   * Returns the expected authority name of this control plane management server. The authority name
   * is used in xDS federation, where xDS clients can specify the authority of an xDS resource.
   *
   * <p>The authority name format assumed in this control plane implementation is of the format
   * {@code [app-name].[namespace].svc.[k8s-dns-cluster-domain]}, e.g., {@code
   * control-plane.xds.svc.cluster.local}. xDS clients must use this format in the {@code
   * authorities} section of their gRPC xDS bootstrap configuration.
   *
   * @see <a
   *     href="https://github.com/cncf/xds/blob/70da609f752ed4544772f144411161d41798f07e/proposals/TP1-xds-transport-next.md#federation">xRFC
   *     TP1: <code>xdstp://</code> structured resource naming, caching and federation support</a>
   * @see <a
   *     href="https://github.com/grpc/proposal/blob/e85c66e48348867937688d89117bad3dcaa6f4f5/A47-xds-federation.md">gRFC
   *     A47: xDS Federation</a>
   */
  public String authorityName() {
    return "%s.%s.svc.%s".formatted(appName(), namespace(), clusterDnsDomain());
  }

  /**
   * Returns the value of the {@code app.kubernetes.io/name} label on this pod. This method reads
   * the value from a file in a volume that was mounted using the Kubernetes downward API.
   */
  String appName() {
    try {
      return Files.readString(
          Path.of(APP_NAME_LABEL_FILEPATH_DOWNWARD_API), StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new ConfigException(
          "Could not read app name label from file " + APP_NAME_LABEL_FILEPATH_DOWNWARD_API, e);
    }
  }

  /**
   * Returns the Kubernetes namespace of this pod. This method first looks for a file in a volume
   * that was mounted using the Kubernetes downward API. If that doesn't exist, it looks for the
   * {@code namespace} file in the {@code serviceaccount} directory. If neither of those files
   * exist, this method throws a runtime exception.
   */
  String namespace() {
    Path namespacePathDownwardApi = Path.of(NAMESPACE_FILEPATH_DOWNWARD_API);
    if (Files.isReadable(namespacePathDownwardApi)) {
      try {
        return Files.readString(namespacePathDownwardApi, StandardCharsets.UTF_8);
      } catch (IOException e) {
        LOG.warn(
            "Could not read pod namespace from file "
                + NAMESPACE_FILEPATH_DOWNWARD_API
                + " even though the file is readable, trying "
                + NAMESPACE_FILEPATH_SERVICE_ACCOUNT
                + " next",
            e);
      }
    }
    try {
      return Files.readString(Path.of(NAMESPACE_FILEPATH_SERVICE_ACCOUNT), StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new ConfigException(
          "Could not read pod namespace from file " + NAMESPACE_FILEPATH_SERVICE_ACCOUNT, e);
    }
  }

  /**
   * Returns the Kubernetes cluster's DNS domain, e.g., {@code cluster.local}.
   *
   * @see <a
   *     href="https://github.com/openjdk/jdk/blob/6765f902505fbdd02f25b599f942437cd805cad1/src/jdk.naming.dns/share/classes/com/sun/jndi/dns/DnsContextFactory.java#L41">
   *     <code>com.sun.jndi.dns.DnsContextFactory</code></a>
   */
  @SuppressWarnings("AddressSelection")
  String clusterDnsDomain() {
    try {
      var k8sSvcFqdn = InetAddress.getByName(KUBERNETES_SERVICE).getCanonicalHostName();
      var domain = k8sSvcFqdn.replaceFirst("^" + Pattern.quote(KUBERNETES_SERVICE + "."), "");
      return domain.endsWith(".") ? domain.substring(0, domain.lastIndexOf(".")) : domain;
    } catch (UnknownHostException e) {
      throw new ConfigException(
          "Could not determine the Kubernetes cluster DNS domain by looking up the FQDN for "
              + KUBERNETES_SERVICE,
          e);
    }
  }

  /**
   * Reads the list of kubecontexts and informer (watch) configurations from a file on the
   * classpath.
   */
  @SuppressWarnings("unchecked")
  @NotNull
  public List<Kubecontext> kubecontexts() {
    var contexts =
        (List<Map<String, Object>>)
            new Load(LoadSettings.builder().build())
                .loadFromInputStream(getClassLoader().getResourceAsStream(INFORMERS_CONFIG_FILE));
    var kubecontexts = new ArrayList<Kubecontext>();
    for (Map<String, Object> context : contexts) {
      var informers = new ArrayList<InformerConfig>();
      for (Map<String, Object> config : (List<Map<String, Object>>) context.get("informers")) {
        informers.add(
            new InformerConfig(
                (String) config.get("namespace"), (List<String>) config.get("services")));
      }
      kubecontexts.add(new Kubecontext((String) context.get("context"), informers));
    }
    LOG.info("Informer configurations by kubecontext: {}", kubecontexts);
    return kubecontexts;
  }

  /** Reads xDS feature flags from a file on the classpath. */
  @SuppressWarnings("unchecked")
  @NotNull
  public XdsFeatures xdsFeatures() {
    var featureMap =
        (Map<String, Boolean>)
            new Load(LoadSettings.builder().build())
                .loadFromInputStream(
                    getClassLoader().getResourceAsStream(XDS_FEATURES_CONFIG_FILE));
    var xdsFeatures = new XdsFeatures(featureMap);
    LOG.info("xDS features: {}", xdsFeatures);
    return xdsFeatures;
  }

  @NotNull
  ClassLoader getClassLoader() {
    try {
      var contextClassLoader = Thread.currentThread().getContextClassLoader();
      if (contextClassLoader != null) {
        return contextClassLoader;
      }
    } catch (Exception e) {
      LOG.warn("Could not get current thread context class loader.", e);
    }
    try {
      var classLoader = this.getClass().getClassLoader();
      if (classLoader != null) {
        return classLoader;
      }
    } catch (Exception e) {
      LOG.warn("Could not get class loader of {} class.", this.getClass().getName(), e);
    }
    var systemClassLoader = ClassLoader.getSystemClassLoader();
    if (systemClassLoader == null) {
      throw new RuntimeException("Could not find class loader to load application configuration.");
    }
    return systemClassLoader;
  }
}
