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

import java.util.Collection;
import java.util.List;

/** Represents a collection of Kubernetes services in a namespace. */
public record InformerConfig(String namespace, Collection<String> services) {

  /** Canonical constructor. */
  public InformerConfig(String namespace, Collection<String> services) {
    this.namespace = namespace == null ? "" : namespace;
    this.services = services == null ? List.of() : List.copyOf(services);
  }
}
