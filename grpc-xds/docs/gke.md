# Google Kubernetes Engine (GKE)

Follow the instructions below to set up a
[Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/docs)
cluster, a container image repository in
[Artifact Registry](https://cloud.google.com/artifact-registry/docs),
and certificate authorities for workload TLS certificates in
[Certificate Authority (CA) Service](https://cloud.google.com/certificate-authority-service/docs).

## Costs

In this document, you use the following billable components of Google Cloud:

- [Artifact Registry](https://cloud.google.com/artifact-registry/pricing)
- [Certificate Authority (CA) Service](https://cloud.google.com/certificate-authority-service/pricing)
- [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/pricing)

To generate a cost estimate based on your projected usage, use the
[pricing calculator](https://cloud.google.com/products/calculator).
New Google Cloud users might be eligible for a free trial.

When you finish the tasks that are described in this document, you can avoid
continued billing by deleting the resources that you created. For more
information, see [Clean up](#clean-up).

## Before you begin

1.  Install the
    [Google Cloud SDK](https://cloud.google.com/sdk/docs/install).

2.  [Configure authorization and a base set of properties](https://cloud.google.com/sdk/docs/initializing)
    for the `gcloud` command line tool. Choose a
    [project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
    that has
    [billing enabled](https://cloud.google.com/billing/docs/how-to/verify-billing-enabled).

3.  Install additional tools that you will use to deploy the xDS control plane
    management service, and the sample gRPC application:

    ```shell
    gcloud components install gke-gcloud-auth-plugin kubectl kustomize skaffold --quiet
    ```
 
4.  Enable the Artifact Registry, GKE, and CA Service APIs:

    ```shell
    gcloud services enable \
      artifactregistry.googleapis.com \
      container.googleapis.com \
      privateca.googleapis.com
    ```

## Artifact Registry setup

1.  Create a container image repository in Artifact Registry:

    ```shell
    gcloud artifacts repositories create REPOSITORY \
      --location=LOCATION \
      --repository-format=docker
    ```

    Replace the following:

    - `REPOSITORY`: the name you want to use for your repository, for instance
      `grpc-xds`.
    - `LOCATION`: an
      [Artifact Registry location](https://cloud.google.com/artifact-registry/docs/repositories/repo-locations),
      for instance `us-central1`.

2.  Configure authentication for `gcloud` and other command-line tools to the
    Artifact Registry host of your repository location:

    ```shell
    gcloud auth configure-docker LOCATION-docker.pkg.dev
    ```

## Creating the Google Kubernetes Engine (GKE) cluster

1.  Create a GKE cluster with a single-zone control plane:

    ```shell
    gcloud container clusters create CLUSTER \
      --enable-ip-alias \
      --enable-mesh-certificates \
      --network default \
      --release-channel rapid \
      --subnetwork default \
      --workload-pool "$(gcloud config get project).svc.id.goog" \
      --zone ZONE \
      --enable-autoscaling \
      --max-nodes 5 \
      --min-nodes 3 \
      --scopes cloud-platform
    ```

    Replace the following:

    - `CLUSTER`: the name you want to use for your cluster, for instance
      `grpc-xds`.
    - `ZONE`: the
      [Compute Engine zone](https://cloud.google.com/compute/docs/regions-zones),
      for the cluster control plane and the default
      [node pool](https://cloud.google.com/kubernetes-engine/docs/concepts/node-pools)
      for instance `us-central1-f`.

2.  Optional: If you want to create a firewall that only allows access to the cluster
    API server from your current public IP address:

    ```shell
    gcloud container clusters update CLUSTER \
      --enable-master-authorized-networks \
      --enable-master-global-access \
      --master-authorized-networks "$(dig TXT +short o-o.myaddr.l.google.com @ns1.google.com | sed 's/"//g')/32" \
      --master-ipv4-cidr "172.16.0.64/28"
    ```

3.  You may find it useful to set the namespace of your current kubeconfig
    context, so you don't need to specify the `xds` namespace for all
    `kubectl` commands:

    ```shell
    kubectl config set-context --current --namespace=xds
    ```

## Workload TLS certificates

To issue workload TLS certificates, you can use certificate authorities in
either
[CA Service](https://cloud.google.com/certificate-authority-service/docs)
or [cert-manager](https://cert-manager.io/docs/).

### Use CA Service

In the Traffic Director document on
[setting up service security with proxyless gRPC](https://cloud.google.com/traffic-director/docs/security-proxyless-setup),
follow the steps in the section titled
[Create certificate authorities to issue certificates](https://cloud.google.com/traffic-director/docs/security-proxyless-setup#configure-cas)
to set up certificate authorities in CA Service to issue workload TLS
certificates to pods on the GKE cluster.

### Use cert-manager

If you want to use a GKE cluster, but you do not want to set up certificate
authorities in CA Service, you can instead use workload TLS certificates
issued by certificate authorities created using
[cert-manager](https://cert-manager.io/docs/).

To do so, run the `make cert-manager` target after creating your GKE cluster.
This target installs cert-manager in the Kubernetes cluster, and creates a
root CA that can issue TLS certificates to any namespace in the cluster.

When you want to deploy the xDS control plane and sample gRPC application to
your GKE cluster, use the `make` targets that end in `-tls-cert-manager`, i.e.
`make run-go-tls-cert-manager` and `make run-java-tls-cert-manager`.

See the [Makefile](../Makefile) for further details.

Advanced: Alternatively, you can rename your kubeconfig context so it matches
the regular expression `kind.*`. This kubeconfig context name will cause
Skaffold to use Kubernetes manifests intended for a kind cluster with
cert-manager.

## Cleaning up

1.  Delete the GKE cluster:

    ```shell
    gcloud container clusters delete CLUSTER --zone ZONE --quiet --async
    ```

2.  Delete the container image repository in Artifact Registry:

    ```shell
    gcloud artifacts repositories delete REPOSITORY \
      --location=LOCATION --async --quiet
    ```
