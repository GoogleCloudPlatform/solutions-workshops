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

set -Eefo pipefail

export SKAFFOLD_BUILD_CONCURRENCY=0
export SKAFFOLD_CACHE_ARTIFACTS=false
export SKAFFOLD_CLEANUP=false
export SKAFFOLD_DETECT_MINIKUBE=false
export SKAFFOLD_INTERACTIVE=false
export SKAFFOLD_SKIP_TESTS=true
export SKAFFOLD_UPDATE_CHECK=false

# language is either "go" or "java". Default to "go".
language=${1:-go}

# If the current context points to a GKE cluster, then deploy to GKE clusters.
# If the current context points to a kind cluster, then deploy to kind clusters.
current_context=$(kubectl config current-context)

# cluster_type will be either "gke_" or "kind".
cluster_type=${current_context:0:4}

contexts=()
while IFS='' read -r line; do contexts+=("$line"); done < <(kubectl config get-contexts --output=name | grep "${cluster_type}.*grpc-xds")

repo_dir=$(git rev-parse --show-toplevel)
base_dir="${repo_dir}/grpc-xds"

context="${contexts[1]}"
echo "Deploying greeter-$language to $context, without port forwarding"
pushd "${base_dir}/greeter-${language}"
skaffold run --kube-context="$context" --port-forward=off --status-check=false
popd

context="${contexts[0]}"
echo "Deploying control-plane-$language and greeter-$language, with port forwarding, to $context"
pushd "$base_dir"
skaffold run --kube-context="$context" --module=go --port-forward=user
popd
