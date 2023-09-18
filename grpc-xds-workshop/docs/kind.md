# kind

[kind](https://kind.sigs.k8s.io/) is a command-line tool for running local
Kubernetes clusters using [podman](https://podman.io/) or
[Docker](https://www.docker.com/).
kind is handy for development purposes if you don't have access to a hosted
Kubernetes cluster and container image registry.

Follow the instructions below to create a multi-node Kubernetes cluster using
[kind](docs/kind.md), with fake
[zone labels (`topology.kubernetes.io/zone`)](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone)
for simulating a cluster with nodes across multiple cloud provider zones.

## Docker Desktop setup

1.  If you want to use kind with Docker Desktop, increase the virtual machine
    CPU and memory allocation, so that the Kubernetes cluster will have
    sufficient capacity.

    Refer to the
    [official documentation](https://docs.docker.com/desktop/settings/mac/#resources)
    for instructions on changing the resource limits.

## Podman setup

If you want to use kind with podman, follow the steps in this section.

1.  Increase the virtual machine CPU and memory allocation, so that the
    Kubernetes cluster will have sufficient capacity.

    For podman, use the
    [`podman machine set` command](https://docs.podman.io/en/latest/markdown/podman-machine-set.1.html)
    to update your existing virtual machine:

    ```shell
    podman machine set --cpus 2 --memory 4096
    ```

    Verify the settings by inspecting the machine:

    ```shell
    podman machine inspect
    ```

2.  Create a symlink called `docker` that points to the `podman` executable:

    ```shell
    ln -s $(which podman) /usr/local/bin/docker
    ```

3.  If you use podman on macOS, verify that the default Docker socket location
    (`/var/run/docker.sock`) redirects to the podman socket location in your
    home directory:

    ```shell
    readlink /var/run/docker.sock
    ```

    On macOS, the output should look similar to this:

    ```
    /Users/$USER/.local/share/containers/podman/machine/podman.sock
    ```

    If this isn't the case (e.g., you installed podman using `brew` rather
    than the `.pkg` installer), use the
    [`podman-mac-helper` tool](https://podman-desktop.io/docs/migrating-from-docker/using-podman-mac-helper)
    to set up the redirection.

4.  Configure kind to use podman by defining and exporting an environment
    variable:

    ```shell
    export KIND_EXPERIMENTAL_PROVIDER=podman
    ```

    Remember to run this command again if you open a new terminal.

## Creating the kind cluster

1.  Create a kind configuration file for a Kubernetes cluster with fake
    [zone labels](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone):

    ```shell
    cat << EOF > kind-cluster-config.yaml
    kind: Cluster
    apiVersion: kind.x-k8s.io/v1alpha4
    name: grpc-xds-workshop
    nodes:
    - role: control-plane
    - role: worker
      labels:
        topology.kubernetes.io/zone: us-central1-a
    - role: worker
      labels:
        topology.kubernetes.io/zone: us-central1-b
    EOF
    ```

2.  Create the kind Kubernetes cluster:

    ```shell
    kind create cluster --config=kind-cluster-config.yaml
    ```

3.  You may find it useful to set the namespace of your current kubeconfig
    context, so you don't need to specify the `xds` namespace for all
    `kubectl` commands:

    ```shell
    kubectl config set-context --current --namespace=xds
    ```

## Cleaning up

1.  When you are done, delete the kind Kubernetes cluster:

    ```shell
    kind delete cluster --name grpc-xds-workshop
    ```
