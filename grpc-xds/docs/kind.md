# kind

[kind](https://kind.sigs.k8s.io/) is a command-line tool for running local
Kubernetes clusters using [podman](https://podman.io/) or
[Docker](https://www.docker.com/).
kind is handy for development purposes if you don't have access to a hosted
Kubernetes cluster and container image registry.

Follow the instructions below to create a multi-node Kubernetes cluster using
[kind](docs/kind.md), with fake
[zone labels (`topology.kubernetes.io/zone`)](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone)
for simulating a cluster with nodes across multiple cloud provider zones, and
with [cert-manager](https://cert-manager.io/docs/) and a root certificate
authority (CA) to issue workload certificates for TLS and mTLS.

## Docker Desktop setup

1.  If you want to use kind with Docker Desktop, increase the virtual machine
    CPU and memory allocation, so that the Kubernetes cluster will have
    sufficient capacity. Set CPUs to 2 (or more), and memory to 4 GB (or more).

    Refer to the
    [official documentation](https://docs.docker.com/desktop/settings/mac/#resources)
    for instructions on changing the resource limits.

## Podman setup

If you want to use kind with podman, follow the steps in this section.

1.  Increase the virtual machine CPU and memory allocation, so that the
    Kubernetes cluster(s) will have sufficient capacity.

    For podman, use the
    [`podman machine set` command](https://docs.podman.io/en/latest/markdown/podman-machine-set.1.html)
    to update your existing virtual machine.

    1.  For running one kind Kubernetes cluster:

        ```shell
        podman machine set --cpus 2 --memory 4096
        ```

    2.  For running two kind Kubernetes clusters:

        ```shell
        podman machine set --cpus 4 --memory 8096
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

1.  Create the kind Kubernetes cluster with fake
    [zone labels](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone),
    install cert-manager, and create a root CA issuer for workload TLS
    certificates:

    ```shell
    make kind-create
    ```

    This command creates a
    [kubeconfig context](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/#context)
    called `kind-grpc-xds`.

    The manifest rendering process assumes that the kubeconfig context name
    matches the regular expression `kind.*`, if you use a kind cluster.

    The command also sets the `xds` namespace as the default namespace
    for the kubeconfig context.

## Cleaning up

1.  When you are done, delete the kind Kubernetes cluster:

    ```shell
    make kind-delete
    ```
