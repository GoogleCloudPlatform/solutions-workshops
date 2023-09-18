# Verify local development setup using Java

Follow the instructions in this document to verify your local workstation
setup prior to the workshop.

## Setup

1.  Install the prerequisite tools as documented in the
    [`README.md`](../README.md).

2.  Create a Kubernetes cluster for the workshop using either
    [Google Kubernetes Engine (GKE)](gke.md) or [`kind`](kind.md).

## Deploy a sample workload

Deploy a sample workload using the toolchain that you will use in the workshop.

1.  Clone the [Skaffold](https://skaffold.dev/docs/) Git repository at a tag
    that matches your Skaffold version:

    ```shell
    git clone --branch=$(skaffold version) --depth=1 --no-checkout --single-branch \
      https://github.com/GoogleContainerTools/skaffold
    ```
    
2.  Create a sparse checkout that contains the `jib-gradle` example:

    ```shell
    cd skaffold
    git sparse-checkout init --cone
    git sparse-checkout set examples/jib-gradle
    git checkout @
    cd examples/jib-gradle
    ```

3.  Update the example to use Java 17, and to remove the base image
    configuration setting:

    ```shell
    sed -i.new 's/"11"/"17"/;/jib\.from\.image/d' build.gradle
    ```

    If no base image is configured, Jib uses its
    [default base image](https://github.com/GoogleContainerTools/jib/blob/v3.3.2-gradle/docs/default_base_image.md)

4.  If you use a remote cluster (e.g., GKE), set the `SKAFFOLD_DEFAULT_REPO`
    environment variable as documented in the [`README.md`](../README.md).

    Skaffold uses this environment variable to
    [determine the full container image name, and to know where to push container images](https://skaffold.dev/docs/environment/image-registries/)
    if you use a remote cluster.

    You can skip this step if you use [kind](kind.md).

5.  Build the container image for the `jib-gradle` example, populate the full
    image name in the Kubernetes resource manifests in the `k8s` directory,
    apply the manifests to your cluster, set up port forwarding to the
    Kubernetes Service resource, and tail the container logs:

    ```shell
    skaffold run --port-forward --skip-tests --tail
    ```

6.  In another terminal window, send a HTTP request to the example application:

    ```shell
    curl -s localhost:8080
    ```

    The output is `Hello World`.

## Troubleshooting

1.  If Skaffold fails to build and push or load the container image, look
    for errors in the Skaffold output. You can increase the verbosity by
    adding the `--verbosity=info` or `--verbosity=debug` flags.

2.  If the container fails to start on your Kubernetes cluster, examine the
    Pod resource:

    ```shell
    kubectl describe $(kubectl get pod --output name --selector 'app=web')
    ```

3.  For other issues, see the Kubernetes documentation on
    [troubleshooting clusters](https://kubernetes.io/docs/tasks/debug/debug-cluster/).

## Clean up

1.  When you are done, delete the Kubernetes resources that you deployed using
    Skaffold:

    ```shell
    skaffold delete
    ```

2.  If you used a GKE cluster and Artifact Registry, delete the container
    image from Artifact Registry:

    ```shell
    gcloud artifacts docker images delete --delete-tags --quiet \
      "$(jq -r '.image' < build/jib-image.json)"
    ```
