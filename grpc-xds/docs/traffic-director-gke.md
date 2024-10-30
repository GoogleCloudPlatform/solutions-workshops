# Traffic Director with Google Kubernetes Engine (GKE)

This document describes how to set up the `greeter-intermediary|leaf` gRPC
applications on a Google Kubernetes Engine (GKE) cluster, and how to connect
the applications to the Traffic Director xDS control plane.

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

- [Certificate Authority (CA) Service](https://cloud.google.com/certificate-authority-service/pricing)
- [Cloud Service Mesh](https://cloud.google.com/service-mesh/pricing)
- [Compute Engine](https://cloud.google.com/compute/all-pricing)
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

1.  Grant the
    [Traffic Director Client role (`roles/trafficdirector.client`)](https://cloud.google.com/iam/docs/understanding-roles#trafficdirector.client)
    role on the project to all identities in your project's identity pool:

    ```shell
    gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member group:$PROJECT_ID.svc.id.goog:/allAuthenticatedUsers/ \
      --role roles/trafficdirector.client
    ```

2.  Grant the
    [Logs Writer role (`roles/logging.logWriter`)](https://cloud.google.com/iam/docs/understanding-roles#logging.logWriter)
    role on the project to all identities in your project's identity pool:

    ```shell
    gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member group:$PROJECT_ID.svc.id.goog:/allAuthenticatedUsers/ \
      --role roles/logging.logWriter
    ```

Instead of granting the roles to all identities in the pool, you can bind
individual Kubernetes ServiceAccounts to IAM service accounts using
[Workload Identity Federation for GKE](https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity),
and grant the Traffic Director Client role to the IAM service accounts.

## Create GKE clusters

Follow the steps in the following sections in the document on [GKE](gke.md):

1.  [VPC network](gke.md#vpc-network)

2.  [Firewall rules](gke.md#firewall-rules)

3.  [Artifact Registry setup](gke.md#artifact-registry-setup)

4.  [Creating Google Kubernetes Engine (GKE) clusters](gke.md#creating-google-kubernetes-engine-gke-clusters)

5.  [Workload TLS certificates using CA service](gke.md#workload-tls-certificates-using-ca-service)

You do _not_ need to follow the steps in the sections on
"Workload identity federation for GKE" and "kubeconfig file".
Those sections are for deploying the sample xDS control plane from this
repository, they are not required when using the managed Traffic Director
xDS control plane.

## Deploy the sample gRPC applications to the first GKE cluster

1.  Create and export an environment variable called `SKAFFOLD_DEFAULT_REPO`
    to point to your container image registry:

    ```shell
    export SKAFFOLD_DEFAULT_REPO=$AR_LOCATION-docker.pkg.dev/$PROJECT_ID/$AR_REPOSITORY
    ```

    Replace the following:

    - `$AR_LOCATION`: the
      [location](https://cloud.google.com/artifact-registry/docs/repositories/repo-locations)
      of your Artifact Registry container image repository, e.g., `us-central1` or `us`.
    - `$PROJECT_ID`: your Google Cloud
      [project ID](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    - `$AR_REPOSITORY`: the name of your Artifact Registry
      [container image repository](https://cloud.google.com/artifact-registry/docs/docker),
      e.g., `grpc-xds`.

2.  Set your `kubectl` context to point to the first GKE cluster:

    ```shell
    kubectl config use-context "gke_${PROJECT_ID}_${REGIONS[@]:0:1}_grpc-xds"
    ```

3.  Deploy the `greeter-intermediary` and `greeter-leaf` gRPC applications,
    with gRPC xDS bootstrap configuration for connecting to your Traffic
    Director control plane tenant. Use either the Go or Java implementations:

    Using Go:

    ```shell
    make run-go-td
    ```

    Using Java:

    ```shell
    make run-java-td
    ```

    Leave the command running to keep the port forwarding active. Complete
    the steps below in a new terminal window.

## Configure Compute Engine API resources

1.  Create global gRPC
    [health checks](https://cloud.google.com/load-balancing/docs/health-check-concepts)
    for the `greeter-intermediary` and `greeter-leaf` applications:

    ```shell
    for app in intermediary leaf ; do
      gcloud compute health-checks create grpc greeter-$app \
        --check-interval 5s \
        --enable-logging \
        --global \
        --grpc-service-name helloworld.Greeter \
        --healthy-threshold 1 \
        --port 50052 \
        --timeout 1s \
        --unhealthy-threshold 1
    done
    ```

2.  Create global
    [backend services](https://cloud.google.com/load-balancing/docs/backend-service)
    for the `greeter-intermediary` and `greeter-leaf` applications:

    ```shell
    for app in intermediary leaf ; do
      gcloud compute backend-services create greeter-$app \
        --global \
        --health-checks greeter-$app \
        --load-balancing-scheme INTERNAL_SELF_MANAGED \
        --protocol GRPC
    done
    ```

3.  Add the zonal network endpoint groups (NEGs) as backends to the backend
    services:

    ```shell
    for app in intermediary leaf ; do
      neg_name=$(kubectl get service greeter-$app -oyaml \
        | yq -r '.metadata.annotations["cloud.google.com/neg-status"]' \
        | yq -r '.network_endpoint_groups.50051')
      kubectl get service greeter-$app --namespace xds --output yaml \
        | yq -r '.metadata.annotations["cloud.google.com/neg-status"]' \
        | yq -r '.zones[]' \
        | xargs -I{} \
        gcloud compute backend-services add-backend greeter-$app \
          --global \
          --network-endpoint-group $neg_name \
          --network-endpoint-group-zone {} \
          --balancing-mode RATE \
          --max-rate-per-endpoint 5
    done
    ```

    This can take a few minutes to complete.

4.  Verify that each backend service has one backend for each zone used by
    your GKE cluster:

    ```shell
    gcloud compute backend-services describe greeter-intermediary --global
    ```

    ```shell
    gcloud compute backend-services describe greeter-leaf --global
    ```

5.  Verify that each backend service has at least one backend with a `HEALTHY`
    endpoint:

    ```shell
    gcloud compute backend-services get-health greeter-intermediary --global
    ```

    ```shell
    gcloud compute backend-services get-health greeter-leaf --global
    ```

## Configure `GrpcRoute` resources using the Network Services API

1.  Create
    [`GrpcRoute`](https://cloud.google.com/service-mesh/docs/reference/network-services/rest/v1/projects.locations.grpcRoutes)
    resources for the `greeter-intermediary` and `greeter-leaf` applications
    in your mesh named `grpc-xds`:

    ```shell
    for app in intermediary leaf ; do
      cat << EOF > grpc-route-greeter-$app.yaml
    name: greeter-$app-grpc-route
    hostnames:
    - greeter-$app
    meshes:
    - projects/$PROJECT_ID/locations/global/meshes/grpc-xds
    rules:
    - action:
        destinations:
        - serviceName: projects/$PROJECT_ID/locations/global/backendServices/greeter-$app
    EOF

      gcloud network-services grpc-routes import greeter-$app \
        --location global \
        --source grpc-route-greeter-$app.yaml
    done
    ```

    The `hostnames` entries become LDS API Listeners, so gRPC clients can
    address the `greeter-intermediary` and `greeter-leaf` applications as
    `xds:///greeter-intermediary` and `xds:///greeter-leaf`, respectively.

## Verify the single-cluster setup

1.  Send a plaintext gRPC request to `greeter-intermediary`:

    ```shell
    make request
    ```

    If you stopped the `make run-[go|java]-td` command, port forwarding is no
    longer active. To set up port forwarding again, run this command to
    port-forward from local port `50055` to port `50051` on a Pod from the
    `greeter-intermediary` Deployment:

    ```shell
    kubectl port-forward --namespace xds deployment/greeter-intermediary 50055:50051
    ```

    If the request fails, see the [Troubleshooting](#troubleshooting) section.

## Deploy the sample gRPC applications to the second GKE cluster

1.  Create and export an environment variable called `SKAFFOLD_DEFAULT_REPO`
    to point to your container image registry:

    ```shell
    export SKAFFOLD_DEFAULT_REPO=$AR_LOCATION-docker.pkg.dev/$PROJECT_ID/$AR_REPOSITORY
    ```

    Replace the following:

    - `$AR_LOCATION`: the
      [location](https://cloud.google.com/artifact-registry/docs/repositories/repo-locations)
      of your Artifact Registry container image repository, e.g., `us-central1` or `us`.
    - `$PROJECT_ID`: your Google Cloud
      [project ID](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    - `$AR_REPOSITORY`: the name of your Artifact Registry
      [container image repository](https://cloud.google.com/artifact-registry/docs/docker),
      e.g., `grpc-xds`.

2.  Set your `kubectl` context to point to the second GKE cluster:

    ```shell
    kubectl config use-context "gke_${PROJECT_ID}_${REGIONS[@]:1:1}_grpc-xds"
    ```

3.  Deploy the `greeter-intermediary` and `greeter-leaf` gRPC applications,
    with gRPC xDS bootstrap configuration for connecting to your Traffic
    Director control plane tenant. Use either the Go or Java implementations:

    Using Go:

    ```shell
    make run-go-td
    ```

    Using Java:

    ```shell
    make run-java-td
    ```

    Leave the command running to keep the port forwarding active. Complete
    the steps below in a new terminal window.

## Update the Compute Engine API resources

1.  Add the new zonal network endpoint groups (NEGs) for the
    `greeter-intermediary` and `greeter-leaf` Kubernetes Services as backends
    to the global backend services:

    ```shell
    for app in intermediary leaf ; do
      neg_name=$(kubectl get service greeter-$app -oyaml \
        | yq -r '.metadata.annotations["cloud.google.com/neg-status"]' \
        | yq -r '.network_endpoint_groups.50051')
      kubectl get service greeter-$app --namespace xds --output yaml \
        | yq -r '.metadata.annotations["cloud.google.com/neg-status"]' \
        | yq -r '.zones[]' \
        | xargs -I{} \
        gcloud compute backend-services add-backend greeter-$app \
          --global \
          --network-endpoint-group $neg_name \
          --network-endpoint-group-zone {} \
          --balancing-mode RATE \
          --max-rate-per-endpoint 5
    done
    ```

    This can take a few minutes to complete.

2.  Verify that each backend service has one backend for each zone used by
    your GKE clusters:

    ```shell
    gcloud compute backend-services describe greeter-intermediary --global
    ```

    ```shell
    gcloud compute backend-services describe greeter-leaf --global
    ```

3.  Verify that each backend service has at least two backends with `HEALTHY`
    endpoints, with at least one `HEALTHY` endpoint in each region:

    ```shell
    gcloud compute backend-services get-health greeter-intermediary --global
    ```

    ```shell
    gcloud compute backend-services get-health greeter-leaf --global
    ```

## Verify the multi-cluster setup

1.  Send ten plaintext gRPC requests to `greeter-intermediary`:

    ```shell
    for i in {0..9} ; do make request ; done
    ```

    Observe that the requests all go to `greeter-leaf` Pods in the same region
    as the `greeter-intermediary` Pod that you are port-forwarding to.

    If you stopped the `make run-[go|java]-td` command, port forwarding is no
    longer active. To set up port forwarding again, run this command to
    port-forward from local port `50055` to port `50051` on a Pod from the
    `greeter-intermediary` Deployment:

    ```shell
    kubectl port-forward --namespace xds deployment/greeter-intermediary 50055:50051
    ```

    If the requests fail, see the [Troubleshooting](#troubleshooting) section.

2.  To verify that `greeter-intermediary` in one GKE cluster can reach
    `greeter-leaf` in the other GKE cluster, scale down to zero replicas the
    `greeter-leaf` Deployment in the GKE cluster where you are
    port-forwarding to the `greeter-intermediary` Pod.

    ```shell
    kubectl scale deployment/greeter-leaf --namespace xds --replicas 0
    ```

    You can display your current `kubectl` context:

    ```shell
    kubectl config current-context
    ```

    For instance, if you are currently port-forwarding from local port `50055`
    to `greeter-intermediary` on port `50051` in the GKE cluster in
    `us-west1`, scale down `greeter-leaf` in the GKE cluster in `us-1`:


3.  Send ten plaintext gRPC requests to `greeter-intermediary`:

    ```shell
    for i in {0..9} ; do make request ; done
    ```

    Observe that the requests all go to `greeter-leaf` Pods in a different
    region to the `greeter-intermediary` Pod where you are port-forwarding.

    This happens because there are no healthy `greeter-leaf` endpoints in the
    same zone or region as the `greeter-intermediary` Pod where you are
    sending requests.

    If the requests fail, see the [Troubleshooting](#troubleshooting) section.

## Configure mTLS

1.  Create a
    [server TLS policy](https://cloud.google.com/service-mesh/docs/reference/network-security/rest/v1beta1/projects.locations.serverTlsPolicies)
    for mTLS, specifying that xDS clients should use the certificates and
    certificate authority (CA) bundles provided by the
    [GKE Workload Identity Certificates controller](https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.locations.clusters#meshcertificates)
    both for locating server certificates, and for validating client
    certificates:

    ```shell
    cat << EOF > policy-server-mtls-gke-spiffe.yaml
    name: projects/$PROJECT_ID/locations/global/serverTlsPolicies/mtls-gke-spiffe
    serverCertificate:
      certificateProviderInstance:
        pluginInstance: google_cloud_private_spiffe
    mtlsPolicy:
      clientValidationCa:
      - certificateProviderInstance:
          pluginInstance: google_cloud_private_spiffe
    EOF

    gcloud network-security server-tls-policies import mtls-gke-spiffe \
      --location global \
      --source policy-server-mtls-gke-spiffe.yaml
    ```

    The gRPC xDS bootstrap JSON file uses `google_cloud_private_spiffe` as the
    name of the certificate provider that references the Pod volume mount of
    the certificates provided by the GKE Workload Identity Certificates
    controller.

2.  Create a workload
    [authorization policy](https://cloud.google.com/service-mesh/docs/reference/network-security/rest/v1beta1/projects.locations.authorizationPolicies)
    that permit requests to port `50051` of the `greeter-intermediary` and
    `greeter-leaf` applications only from gRPC clients that can present a TLS
    certificate with a SPIFFE ID that matches one of the `principals` listed
    in the policy:

    ```shell
    cat << EOF > authorization-policy-greeter.yaml
    name: greeter
    action: ALLOW
    rules:
    - sources:
      - principals:
        - spiffe://$PROJECT_ID.svc.id.goog/ns/host-certs/sa/host
        - spiffe://$PROJECT_ID.svc.id.goog/ns/xds/sa/*
      destinations:
      - hosts:
        - greeter-intermediary
        - greeter-leaf
        ports:
        - 50051
    EOF

    gcloud network-security authorization-policies import greeter \
      --location global \
      --source authorization-policy-greeter.yaml
    ```

3.  Create an
    [endpoint policy](https://cloud.google.com/service-mesh/docs/reference/network-services/rest/v1beta1/projects.locations.endpointPolicies)
    that attaches the server TLS policy for mTLS, and the workload
    authorization policy, to port `50051` of all xDS clients that match the
    `endpointMatcher`. The policy below matches gRPC servers that present the
    label `component` with the value `greeter` as part of its node metadata in
    the gRPC xDS bootstrap JSON config:

    ```shell
    cat << EOF > endpoint-policy-mtls-gke-spiffe-greeter.yaml
    name: mtls-gke-spiffe-greeter
    type: GRPC_SERVER
    endpointMatcher:
      metadataLabelMatcher:
        metadataLabelMatchCriteria: MATCH_ALL
        metadataLabels:
        - labelName: component
          labelValue: greeter
    trafficPortSelector:
      ports:
      - "50051"
    serverTlsPolicy: projects/$PROJECT_ID/locations/global/serverTlsPolicies/mtls-gke-spiffe
    authorizationPolicy: projects/$PROJECT_ID/locations/global/authorizationPolicies/greeter
    EOF

    gcloud network-services endpoint-policies import mtls-gke-spiffe-greeter \
      --location global \
      --source endpoint-policy-mtls-gke-spiffe-greeter.yaml
    ```

4.  Create a
    [client TLS policy](https://cloud.google.com/service-mesh/docs/reference/network-security/rest/v1beta1/projects.locations.clientTlsPolicies)
    for mTLS, specifying that xDS clients should use the certificates and
    certificate authority (CA) bundles provided by the
    [GKE Workload Identity Certificates controller](https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.locations.clusters#meshcertificates)
    both for locating client certificates, and for validating server
    certificates:

    ```shell
    cat << EOF > policy-client-mtls-gke-spiffe.yaml
    name: projects/$PROJECT_ID/locations/global/clientTlsPolicies/mtls-gke-spiffe
    clientCertificate:
      certificateProviderInstance:
        pluginInstance: google_cloud_private_spiffe
    serverValidationCa:
    - certificateProviderInstance:
        pluginInstance: google_cloud_private_spiffe
    EOF

    gcloud network-security client-tls-policies import mtls-gke-spiffe \
      --location global \
      --source policy-client-mtls-gke-spiffe.yaml
    ```

5.  Add configuration for
    [server authorization, also known as SAN checks](https://github.com/grpc/proposal/blob/deaf1bcf248d1e48e83c470b00930cbd363fab6d/A29-xds-tls-security.md#server-authorization-aka-subject-alt-name-checks)
    to the backend services for `greeter-intermediary` and `greeter-leaf`:

    ```shell
    for app in intermediary leaf ; do
      tmp_file="$(mktemp)"

      gcloud compute backend-services export greeter-$app \
        --destination "$tmp_file" \
        --global

      cat << EOF >> "$tmp_file"
    securitySettings:
      clientTlsPolicy: projects/$PROJECT_ID/locations/global/clientTlsPolicies/mtls-gke-spiffe
      subjectAltNames:
      - "spiffe://$PROJECT_ID.svc.id.goog/ns/xds/sa/greeter-$app"
    EOF

      gcloud compute backend-services import greeter-$app \
        --source "$tmp_file" \
        --global \
        --quiet
    done
    ```

6.  Create a client certificate and CA bundle for your developer workstation:

    ```shell
    make host-certs
    ```

7.  Send an mTLS gRPC request to `greeter-intermediary`:

    ```shell
    make request-mtls-noverify
    ```

## Troubleshooting

1.  Deploy the troubleshooting Pod:

    ```shell
    make run-bastion-td
    ```

2.  Run a shell in the troubleshooting Pod:

    ```shell
    make troubleshoot
    ```

    Run the rest of the commands in this section from the troubleshooting Pod.

3.  List the pods in the `xds` namespace, including their IP addresses:

    ```shell
    kubectl get pods --namespace xds --output wide
    ```

    Make a note of the IP address of the Pod of the `greeter-intermediary`
    Deployment. Its `NAME` starts with `greeter-intermediary-`.

4.  Examine the xDS resources cached by the `greeter-intermediary` Pod:

    ```shell
    grpcdebug IP:50052 xds status
    ```

    Replace `IP` with the IP address of the Pod of the `greeter-intermediary`
    Deployment that you found in the previous step.

5.  Print the full details of the LDS resources cached by the
    `greeter-intermediary` Pod as YAML:

    ```shell
    grpcdebug IP:50052 xds config --type=LDS | yq --input-format=json --prettyPrint
    ```

    Replace `IP` with the IP address of the Pod of the `greeter-intermediary`
    Deployment that you found previously.

6.  If you encounter issues with the mTLS configuration or the workload
    authorization policies, see the
    [Troubleshooting section of the document on setting up service security with proxyless gRPC](https://cloud.google.com/service-mesh/docs/service-routing/security-proxyless-setup#troubleshoot-proxyless).

## Clean up

1.  Set up environment variables:

    ```shell
    PROJECT_ID="$(gcloud config get project 2> /dev/null)"
    PROJECT_NUMBER="$(gcloud projects describe $PROJECT_ID --format 'value(projectNumber)')"
    REGIONS=(us-central1 us-west1)
    AR_LOCATION="${REGIONS[@]:0:1}"
    AR_REPOSITORY=grpc-xds
    CAS_ROOT_LOCATION="${REGIONS[@]:0:1}"
    ```

2.  Delete the Network Services and Network Security API resources:

    ```shell
    for app in intermediary leaf ; do
      gcloud network-services grpc-routes delete greeter-$app --location global
      gcloud compute backend-services delete greeter-$app --global --quiet
      gcloud compute health-checks delete greeter-$app --quiet
      for region in "${REGIONS[@]}"; do
        kubectl --context "gke_${PROJECT_ID}_${region}_grpc-xds" annotate service/greeter-$app cloud.google.com/neg-
      done
    done
    gcloud network-services endpoint-policies delete mtls-gke-spiffe-greeter --location global --quiet
    gcloud network-security authorization-policies delete greeter --location global --quiet
    gcloud network-security client-tls-policies delete mtls-gke-spiffe --location global --quiet
    gcloud network-security server-tls-policies delete mtls-gke-spiffe --location global --quiet
    ```

3.  Follow the steps in the
    [Clean up section of the document on deploying to GKE](gke.md#clean-up).
