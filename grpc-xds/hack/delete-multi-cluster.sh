#!/usr/bin/env bash
#
# Copyright 2024 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -Eefuo pipefail

delete_resources () {
  echo "Deleting resources from context $1:"
  kubectl delete --context="$1" --ignore-not-found --namespace=xds deployment control-plane greeter-intermediary greeter-leaf
  kubectl delete --context="$1" --ignore-not-found --namespace=xds service control-plane greeter-intermediary greeter-leaf
  kubectl delete --context="$1" --ignore-not-found --namespace=xds configmaps --selector='app.kubernetes.io/part-of=grpc-xds'
}
export -f delete_resources

# If the current context points to a GKE cluster, then delete from GKE clusters.
# If the current context points to a kind cluster, then delete from kind clusters.
current_context=$(kubectl config current-context)

# cluster_type will be either "gke_" or "kind"
cluster_type=${current_context:0:4}

kubectl config get-contexts --output=name | grep "${cluster_type}.*grpc-xds" | xargs -I{} -L1 bash -c "delete_resources {}"
