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
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
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

  /** Default management server port. Override with the <code>PORT</code> environment variable. */
  private static final int DEFAULT_SERVING_PORT = 50051;

  /**
   * Default health checking port. Override with the <code>HEALTH_PORT</code> environment variable.
   */
  private static final int DEFAULT_HEALTH_PORT = 50052;

  /** The file that lists the K8s Services whose EndpointSlices should be watched. */
  private static final String INFORMERS_CONFIG_FILE = "config/informers.yaml";

  /** The file that contains xDS feature toggles. */
  private static final String XDS_FEATURES_CONFIG_FILE = "config/xds_features.yaml";

  /** Returns the port number that the management server should listen on for xDS requests. */
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

  /** Returns the port number that the management server should listen on for health requests. */
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
