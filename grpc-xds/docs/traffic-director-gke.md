# Traffic Director with Google Kubernetes Engine (GKE)

This document describes how to set up the `greeter-intermediary|leaf` gRPC
applications on a Google Kubernetes Engine (GKE) cluster, and how to connect
them to the Traffic Director xDS control plane.

# Traffic Director and Cloud Service Mesh

[Cloud Service Mesh](https://cloud.google.com/service-mesh/docs/overview) is a
product from Google Cloud that enables managed, observable, and secure
communication among your services.

[Traffic Director](https://cloud.google.com/blog/topics/developers-practitioners/traffic-director-explained)
is a managed xDS control plane that is part of the Cloud Service Mesh product.
Traffic Director is available as a global service. Your workloads can connect
to Traffic Director from Google Cloud, on-prem, and from other clouds, and
the workloads can run on Kubernetes clusters, VMs, and serverless compute
([Cloud Run](https://cloud.google.com/service-mesh/docs/configure-cloud-service-mesh-for-cloud-run)).
For further details, see
[Supported platforms](https://cloud.google.com/service-mesh/docs/supported-platforms).

## Costs

In this document, you use the following billable components of Google Cloud:

- [Cloud Service Mesh](https://cloud.google.com/service-mesh/pricing)
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

3.  Install `kubectl`, and authentication plugins for `kubectl`:

    ```shell
    gcloud components install kubectl gke-gcloud-auth-plugin kubectl-oidc
    ```

    The `gke-gcloud-auth-plugin` plugin enables `kubectl` to authenticate to
    GKE clusters using credentials obtained using `gcloud`.

4.  Set the Google Cloud project you want to use:

    ```shell
    gcloud config set project PROJECT_ID
    ```

    Replace `PROJECT_ID` with the
    [project ID](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
    of the Google Cloud project you want to use.

5.  Enable the Network Security, Network Services, and Traffic Director APIs:

    ```shell
    gcloud services enable \
      networksecurity.googleapis.com \
      networkservices.googleapis.com \
      trafficdirector.googleapis.com
    ```

## Provision a Traffic Director service mesh

1.  Create a file that defines the name of your service mesh:

    ```shell
    cat << EOF > mesh-grpc-xds.yaml
    name: grpc-xds
    EOF
    ```

2.  Create your Traffic Director xDS control plane tenant:

    ```shell
    gcloud network-services meshes import grpc-xds \
      --location global \
      --source mesh-grpc-xds.yaml
    ```

3.  View the service mesh you just provisioned

    ```shell
    gcloud network-services meshes describe grpc-xds --location global
    ```

## Grant access to the Traffic Director xDS control plane

Your workload need access to stream xDS resources from your Traffic Director
xDS control plane tenant.

1.  Create an IAM service account for your workloads:

    ```shell
    GSA_TD_CLIENT=td-client

    gcloud iam service-accounts create $GSA_TD_CLIENT \
      --display-name "Traffic Director xDS Client"
    ```

2.  Grant the
    [Traffic Director Client role (`roles/trafficdirector.client`)](https://cloud.google.com/iam/docs/understanding-roles#trafficdirector.client)
    role on the project to all identities in your project's identity pool:

    ```shell
    gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member group:$PROJECT_ID.svc.id.goog:/allAuthenticatedUsers/ \
      --role roles/trafficdirector.client
    ```

TBC

