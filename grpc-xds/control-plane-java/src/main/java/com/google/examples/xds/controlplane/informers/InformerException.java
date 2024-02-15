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

/** Unchecked exception used to wrap checked exceptions from the Kubernetes client library. */
public class InformerException extends RuntimeException {

  public InformerException(String message) {
    super(message);
  }

  public InformerException(String message, Throwable cause) {
    super(message, cause);
  }
}
