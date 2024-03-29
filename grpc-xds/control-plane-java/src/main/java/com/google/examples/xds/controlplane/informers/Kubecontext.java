// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.examples.xds.controlplane.informers;

import java.util.Collection;
import java.util.List;
import org.jetbrains.annotations.Nullable;

/** Kubecontext represents a kubeconfig context, containing a list of {@link InformerConfig}s. */
public record Kubecontext(String context, Collection<InformerConfig> informers) {

  /** Canonical constructor. */
  public Kubecontext(@Nullable String context, @Nullable Collection<InformerConfig> informers) {
    this.context = context == null ? "" : context;
    this.informers = informers == null ? List.of() : List.copyOf(informers);
  }
}
