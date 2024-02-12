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

if ! command -v docker &> /dev/null ; then
  >&2 echo "You must have \`docker\` on your PATH, either as the Docker CLI binary, or as a symlink to \`podman\`".
  exit 1
fi

# Workaround for
# https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
KIND_EXPERIMENTAL_PROVIDER=${KIND_EXPERIMENTAL_PROVIDER:-}
if [ "$KIND_EXPERIMENTAL_PROVIDER" == "podman" ] ; then
  podman machine ssh \
    'grep -v "fs.inotify.max_user_[instances|watches]" /etc/sysctl.conf | sudo tee /etc/sysctl.conf > /dev/null &&
     echo "fs.inotify.max_user_watches=1048576" | sudo tee -a /etc/sysctl.conf > /dev/null &&
     echo "fs.inotify.max_user_instances=8192" | sudo tee -a /etc/sysctl.conf > /dev/null &&
     sudo sysctl -p /etc/sysctl.conf'
elif [ "$(uname -s)" != "Linux" ] ; then
  # Assuming Docker Desktop
  docker run -it --privileged --pid=host debian:stable nsenter -t 1 -m -u -n -i sh -c \
    'grep -v "fs.inotify.max_user_[instances|watches]" /etc/sysctl.conf | sudo tee /etc/sysctl.conf > /dev/null &&
     echo "fs.inotify.max_user_watches=1048576" | sudo tee -a /etc/sysctl.conf > /dev/null &&
     echo "fs.inotify.max_user_instances=8192" | sudo tee -a /etc/sysctl.conf > /dev/null &&
     sudo sysctl -p /etc/sysctl.conf'
else
  # In case of Docker Engine on a Linux host, don't just change the host.
  echo "******************************************************************************************
* It looks like you are using Docker Engine on a machine running Linux.                  *
* If you encounter problems starting the two Kubernetes clusters,                        *
* update your inotify limis as per these instructions:                                   *
* https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files *
******************************************************************************************"
fi

# Speed up `skaffold` commands.
export SKAFFOLD_DETECT_MINIKUBE=false
export SKAFFOLD_INTERACTIVE=false
export SKAFFOLD_SKIP_TESTS=true
export SKAFFOLD_UPDATE_CHECK=false

# Create the first cluster.
kind create cluster --config=docs/kind-cluster-config.yaml
kubectl config set-context --current --namespace=xds
skaffold run --filename=k8s/cert-manager/skaffold.yaml --module=cert-manager
skaffold run --filename=k8s/cert-manager/skaffold.yaml --module=root-ca

# Create the second cluster.
kind create cluster --config=docs/kind-cluster-config-2.yaml
kubectl config set-context --current --namespace=xds
skaffold run --filename=k8s/cert-manager/skaffold.yaml --module=cert-manager
skaffold run --filename=k8s/cert-manager/skaffold.yaml --module=root-ca-external

# Set up routing rules between pod and service IP ranges of the two clusters.
#
# Cluster 1 -> Cluster 2
for node in $(kind get nodes --name=grpc-xds) ; do
  # Add static routes to the pod subnets of each node in the other cluster.
  # `ip route replace` performs an upsert, so using that instead of `ip route add`.
  kubectl --context=kind-grpc-xds-2 get nodes --output=jsonpath='{range .items[*]}{"ip route replace "}{.spec.podCIDR}{" via "}{.status.addresses[?(@.type=="InternalIP")].address}{"\n"}{end}' \
    | xargs -L1 docker exec "$node"
  # Add a static route to the service subnet in the other cluster, pointing at the control-plane node (could be any node).
  cluster2_service_cidr="10.220.0.0/16" # Match networking.serviceSubnet from `kind-cluster-config-2.yaml`
  cluster2_control_plane_node_ip=$(kubectl --context=kind-grpc-xds-2 get nodes --selector=node-role.kubernetes.io/control-plane --output=jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
  docker exec "$node" ip route replace "$cluster2_service_cidr" via "$cluster2_control_plane_node_ip"
done
#
# Cluster 2 -> Cluster 1
for node in $(kind get nodes --name=grpc-xds-2) ; do
  # Add static routes to the pod subnets of each node in the other cluster.
  # `ip route replace` performs an upsert, so using that instead of `ip route add`.
  kubectl --context=kind-grpc-xds get nodes --output=jsonpath='{range .items[*]}{"ip route replace "}{.spec.podCIDR}{" via "}{.status.addresses[?(@.type=="InternalIP")].address}{"\n"}{end}' \
    | xargs -L1 docker exec "$node"
  # Add a static route to the service subnet in the other cluster, pointing at the control-plane node (could be any node).
  cluster1_service_cidr="10.110.0.0/16" # Match networking.serviceSubnet from `kind-cluster-config.yaml`
  cluster1_control_plane_node_ip=$(kubectl --context=kind-grpc-xds get nodes --selector=node-role.kubernetes.io/control-plane --output=jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
  docker exec "$node" ip route replace "$cluster1_service_cidr" via "$cluster1_control_plane_node_ip"
done

# Patch CoreDNS to forward DNS requests between the two clusters (cluster.local and cluster2.local)
# See https://coredns.io/plugins/forward/ and 
# https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/#configuration-of-stub-domain-and-upstream-nameserver-using-coredns
#
# Cluster 1 -> Cluster 2
cluster2_kube_dns_ip=$(kubectl --context=kind-grpc-xds-2 --namespace=kube-system get service kube-dns --output=go-template='{{.spec.clusterIP}}')
corefile_cluster1=$(mktemp)
kubectl --context=kind-grpc-xds --namespace=kube-system get configmap coredns --output=go-template='{{index .data "Corefile"}}' > "$corefile_cluster1"
cat << EOF >> "$corefile_cluster1"
cluster2.local:53 {
    errors
    cache 30
    forward . $cluster2_kube_dns_ip
}
EOF
kubectl --context=kind-grpc-xds --namespace=kube-system patch configmap coredns --type=merge \
  --patch-file=<(kubectl create configmap coredns --from-file=Corefile=$corefile_cluster1 --dry-run=client --output=yaml --show-managed-fields=false)
#
# Cluster 2 -> Cluster 1
cluster1_kube_dns_ip=$(kubectl --context=kind-grpc-xds --namespace=kube-system get service kube-dns --output=go-template='{{.spec.clusterIP}}')
corefile_cluster2=$(mktemp)
kubectl --context=kind-grpc-xds-2 --namespace=kube-system get configmap coredns --output=go-template='{{index .data "Corefile"}}' > "$corefile_cluster2"
cat << EOF >> "$corefile_cluster2"
cluster.local:53 {
    errors
    cache 30
    forward . $cluster1_kube_dns_ip
}
EOF
kubectl --context=kind-grpc-xds-2 --namespace=kube-system patch configmap coredns --type=merge \
  --patch-file=<(kubectl create configmap coredns --from-file=Corefile=$corefile_cluster2 --dry-run=client --output=yaml --show-managed-fields=false)

# Generate kubeconfig files for the two clusters, for internal cluster control plane connectivity.
kind get kubeconfig --internal --name=grpc-xds \
  | yq eval '(.current-context = "grpc-xds") | (.contexts[0].name = "grpc-xds")' \
  > control-plane-go/k8s/components/kubeconfig/kubeconfig-1.yaml
# When merging kubeconfigs, the first `current-context` will take precedence.
# So changing `current-context` for the second cluster kubeconfig is actually redundant.
kind get kubeconfig --internal --name=grpc-xds-2 \
  | yq eval '(.current-context = "grpc-xds-2") | (.contexts[0].name = "grpc-xds-2")' \
  > control-plane-go/k8s/components/kubeconfig/kubeconfig-2.yaml

# Set the current kubectl context to the first cluster.
kubectl config use-context kind-grpc-xds
