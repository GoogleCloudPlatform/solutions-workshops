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
- [Cloud DNS](https://cloud.google.com/dns/pricing)
- [Cloud NAT](https://cloud.google.com/nat/pricing)
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

3.  Install the `gke-gcloud-auth-plugin` plugin for `kubectl`. This plugin
    enables `kubectl` to authenticate to GKE clusters using credentials
    obtained using `gcloud`:

    ```shell
    gcloud components install gke-gcloud-auth-plugin
    ```

4.  Install additional tools that you will use to deploy the xDS control plane
    management service, and the sample gRPC application:

    - [kubectl](https://kubernetes.io/docs/reference/kubectl/)
    - [Kustomize](https://kustomize.io/) v4.5.5 or later
    - [Skaffold](https://skaffold.dev/) v2.10.1 or later
    - [gRPCurl](https://github.com/fullstorydev/grpcurl) v1.9.1 or later
    - [yq](https://mikefarah.gitbook.io/yq/) v4.41.1 or later

5.  Set the Google Cloud project you want to use:

    ```shell
    gcloud config set project PROJECT_ID
    ```

    Replace `PROJECT_ID` with the
    [project ID](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
    of the Google Cloud project you want to use.

6.  Enable the Artifact Registry, GKE, Cloud DNS, and CA Service APIs:

    ```shell
    gcloud services enable \
      artifactregistry.googleapis.com \
      container.googleapis.com \
      dns.googleapis.com \
      privateca.googleapis.com
    ```

## Firewall rules

1.  Create a firewall rule that allows all TCP, UDP, and ICMP traffic within
    your VPC network

    ```shell
    gcloud compute firewall-rules create allow-internal \
        --allow tcp,udp,icmp \
        --network default \
        --source-ranges "10.0.0.0/8"
    ```

2.  Create a firewall rule that allows health checks from Google Cloud Load
    Balancers to Compute Engine instances in your VPC network that have the
    `allow-health-checks` network tag.

    ```shell
    gcloud compute firewall-rules create allow-health-checks \
        --allow tcp \
        --network default \
        --source-ranges "35.191.0.0/16,130.211.0.0/22" \
        --target-tags allow-health-checks
    ```

## Cloud NAT setup

[Cloud NAT](https://cloud.google.com/nat/docs/) enables internet access from
GKE cluster nodes and pods, even if the nodes do not have public IP addresses.

1.  Define the
    [Compute Engine region(s)](https://cloud.google.com/compute/docs/regions-zones)
    where you will create GKE clusters:

    ```shell
    REGIONS=(us-central1 us-west1)
    ```

2.  Create Cloud Routers:

    ```shell
    for region in "${REGIONS[@]}"; do
      gcloud compute routers create grpc-xds \
        --network default \
        --region "$region"
    done
    ```

3.  Create Cloud NAT gateways:

    ```shell
    for region in "${REGIONS[@]}"; do
      gcloud compute routers nats create grpc-xds \
        --auto-allocate-nat-external-ips \
        --nat-all-subnet-ip-ranges \
        --region "$region" \
        --router grpc-xds
    done
    ```

## Artifact Registry setup

1.  Define environment variables that you use when creating the Artifact
    Registry container image repository:

    ```shell
    AR_LOCATION="${REGIONS[@]:0:1}"
    PROJECT_ID="$(gcloud config get project 2> /dev/null)"
    PROJECT_NUMBER="$(gcloud projects describe $PROJECT_ID --format 'value(projectNumber)')"
    ```

    Note the following about the environment variables:

    - `AR_LOCATION`: an
      [Artifact Registry location](https://cloud.google.com/artifact-registry/docs/repositories/repo-locations).
      In order to reduce network cost, you can use one of the regions that you
      will use for your GKE clusters, or a multi-region location such as `us`.
    - `PROJECT_ID`: the project ID of your
      [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    - `PROJECT_NUMBER`: the automatically generate project number of your
      [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).

2.  Create a container image repository called `grpc-xds` in Artifact Registry:

    ```shell
    gcloud artifacts repositories create grpc-xds \
      --location "$AR_LOCATION" \
      --repository-format docker
    ```

3.  Configure authentication for `gcloud` and other command-line tools to the
    Artifact Registry host of your repository location:

    ```shell
    gcloud auth configure-docker "${AR_LOCATION}-docker.pkg.dev"
    ```

4.  Grant the
    [Artifact Registry Reader role](https://cloud.google.com/artifact-registry/docs/access-control#roles)
    on the container image repository to the
    [Compute Engine default service account](https://cloud.google.com/compute/docs/access/service-accounts#default_service_account):

    ```shell
    gcloud artifacts repositories add-iam-policy-binding grpc-xds \
      --location "$AR_LOCATION" \
      --member "serviceAccount:${PROJECT_NUMBER}-compute@developer.gserviceaccount.com" \
      --role roles/artifactregistry.reader
    ```

## Creating the Google Kubernetes Engine (GKE) cluster

1.  Define an environment variable that you use when creating the GKE clusters:

    ```shell
    PROJECT_ID="$(gcloud config get project 2> /dev/null)"
    ```

    `PROJECT_ID` is the project ID of your
    [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).

2.  Create GKE clusters:

    ```shell
    iter=1
    for region in "${REGIONS[@]}"; do
      gcloud container clusters create grpc-xds \
        --cluster-dns clouddns \
        --cluster-dns-domain "cluster${iter%1}.example.com" \
        --cluster-dns-scope vpc \
        --enable-dataplane-v2 \
        --enable-ip-alias \
        --enable-l4-ilb-subsetting \
        --enable-master-global-access \
        --enable-mesh-certificates \
        --enable-private-nodes \
        --location "$region" \
        --master-ipv4-cidr "172.16.${iter}.64/28" \
        --network default \
        --release-channel rapid \
        --subnetwork default \
        --workload-pool "${PROJECT_ID}.svc.id.goog" \
        --enable-autoscaling \
        --max-nodes 5 \
        --min-nodes 2 \
        --num-nodes 2 \
        --scopes cloud-platform,userinfo-email \
        --tags allow-health-checks,grpc-xds-node \
        --workload-metadata GKE_METADATA
      kubectl config set-context --current --namespace=xds
      iter=$(expr $iter + 1)
    done
    kubectl config use-context "gke_${PROJECT_ID}_${REGIONS[@]:0:1}_grpc-xds"
    ```

3.  Allows access to the cluster API servers from your current public IP
    address and private IP addresses in your VPC network:

    ```shell
    public_ip="$(dig TXT +short o-o.myaddr.l.google.com @ns1.google.com | sed 's/"//g')"
    for region in "${REGIONS[@]}"; do
      gcloud container clusters update grpc-xds \
        --enable-master-authorized-networks \
        --location "$region" \
        --master-authorized-networks "${public_ip}/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
    done
    ```

    If you want to allow access from other IP address ranges, or if you use
    [non-RFC1918 IPv4 address ranges](https://cloud.google.com/vpc/docs/subnets#valid-ranges)
    for your GKE cluster nodes and/or Pods, add those address ranges to the
    `--master-authorized-networks` flag.

## Workload identity federation for GKE

To enable the xDS control plane to communicate with another GKE cluster
control plane, use
[workload identity federation for GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
to enable the xDS control plane Kubernetes service account to authenticate as
an IAM service account without storing long-lived credentials.

1.  Define the name of the IAM service account as an environment variable:

    ```shell
    GSA=xds-control-plane
    ```

    You can use a different name if you like.

2.  Create the IAM service account:

    ```shell
    gcloud iam service-accounts create $GSA --display-name "xDS control plane"
    ```

3.  Grant the
    [Workload Identity User role](https://cloud.google.com/iam/docs/understanding-roles#iam.workloadIdentityUser)
    on the IAM service account to the `control-plane` Kubernetes service
    account in the `xds` namespace:

    ```shell
    gcloud iam service-accounts add-iam-policy-binding "${GSA}@${PROJECT_ID}.iam.gserviceaccount.com" \
      --member "serviceAccount:${PROJECT_ID}.svc.id.goog[xds/control-plane]" \
      --role roles/iam.workloadIdentityUser
    ```

4.  Create a patch to bind the xDS control plane Kubernetes service account to
    the IAM service account, by adding the workload identity federation for
    GKE annotation:

    ```shell
    echo "$(kubectl annotate --dry-run=client --local --output=yaml \
      --filename=k8s/control-plane/base/service-account.yaml \
      iam.gke.io/gcp-service-account=${GSA}@${PROJECT_ID}.iam.gserviceaccount.com)" \
      > k8s/control-plane/components/gke-workload-identity/patch-gke-workload-identity.yaml
    ```

5.  Create a Kubernetes ClusterRoleBinding manifest that grants a Kubernetes
    ClusterRole named `endpointslices-reader` to the IAM service account. The
    ClusterRole provides permissions to list and get EndpointSlices:

    ```shell
    cat << EOF > k8s/control-plane/components/gke-workload-identity/cluster-role-binding-gcp.yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: control-plane-endpointslices-reader-gcp
      labels:
        app.kubernetes.io/component: control-plane
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: endpointslices-reader
    subjects:
    - apiGroup: rbac.authorization.k8s.io
      kind: User
      name: ${GSA}@${PROJECT_ID}.iam.gserviceaccount.com
    EOF
    ```

5.  Apply the Kubernetes `endpointslices-reader` ClusterRole and
    `control-plane-endpointslices-reader-gcp` ClusterRoleBinding to the GKE
    clusters:

    ```shell
    for region in "${REGIONS[@]}"; do
      kubectl apply --context "gke_${PROJECT_ID}_${region}_grpc-xds" \
        --filename k8s/control-plane/base/cluster-role.yaml
      kubectl apply --context "gke_${PROJECT_ID}_${region}_grpc-xds" \
        --filename k8s/control-plane/components/gke-workload-identity/cluster-role-binding-gcp.yaml
    done
    ```

## kubeconfig file

To enable the xDS control plane management server to access the API server of
both Kubernetes clusters, create a
[kubeconfig file](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
with a [Config](https://kubernetes.io/docs/reference/config-api/kubeconfig.v1/)
resource that contains a
[context](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/#context)
for each cluster:

```shell
REGIONS=(us-central1 us-west1)
PROJECT_ID="$(gcloud config get project 2> /dev/null)"

kubeconfig_dir=k8s/control-plane/components/kubeconfig
kubeconfig_file="${kubeconfig_dir}/kubeconfig.yaml"
rm -f "$kubeconfig_file"

iter=1
for region in "${REGIONS[@]}"; do
  cluster="grpc-xds-${region}"
  context="grpc-xds-${iter}"
  context="${context%-1}"

  kubectl config set-context "$context" \
    --cluster "$cluster" \
    --user user \
    --kubeconfig "$kubeconfig_file"

  cluster_ca_file="$(mktemp)"
  gcloud container clusters describe grpc-xds --format='value(masterAuth.clusterCaCertificate)' --location=$region | base64 --decode > "$cluster_ca_file"
  kubectl config set-cluster "$cluster" \
    --certificate-authority "$cluster_ca_file" \
    --embed-certs \
    --server "https://$(gcloud container clusters describe grpc-xds --format='value(privateClusterConfig.privateEndpoint)' --location=$region)" \
    --kubeconfig "$kubeconfig_file"
  rm -f "$cluster_ca_file"

  iter=$(expr $iter + 1)
done

kubectl config set-credentials user \
  --auth-provider google \
  --kubeconfig "$kubeconfig_file"

kubectl config use-context grpc-xds --kubeconfig "$kubeconfig_file"
```

You must recreate the kubeconfig file, by running the snippet above again,
if you
[rotate your GKE cluster credentials](https://cloud.google.com/kubernetes-engine/docs/how-to/credential-rotation)
or
[rotate your GKE cluster control plane IP address](https://cloud.google.com/kubernetes-engine/docs/how-to/ip-rotation).

## Workload TLS certificates using CA Service

In the Cloud Service Mesh document on
[setting up service security with proxyless gRPC](https://cloud.google.com/service-mesh/docs/service-routing/security-proxyless-setup),
follow the steps in the section titled
[Create certificate authorities to issue certificates](https://cloud.google.com/service-mesh/docs/service-routing/security-proxyless-setup#configure-cas)
to set up certificate authorities in CA Service to issue workload TLS
certificates to pods on the GKE clusters.

Create the `WorkloadCertificateConfig` and `TrustConfig` resources in all of
the GKE clusters.

Create the root CA:

```shell
CAS_ROOT_LOCATION="${REGIONS[@]:0:1}"
PROJECT_ID="$(gcloud config get project 2> /dev/null)"
PROJECT_NUMBER="$(gcloud projects describe $PROJECT_ID --format 'value(projectNumber)')"

gcloud privateca pools create grpc-xds-root \
  --location "$CAS_ROOT_LOCATION" \
  --tier enterprise

gcloud privateca roots create grpc-xds \
  --auto-enable \
  --pool grpc-xds-root \
  --subject "CN=grpc-xds-root-ca, O=Example LLC" \
  --key-algorithm ec-p256-sha256 \
  --max-chain-length 1 \
  --location "$CAS_ROOT_LOCATION"

gcloud privateca pools add-iam-policy-binding grpc-xds-root \
  --location "$CAS_ROOT_LOCATION" \
  --role roles/privateca.auditor \
  --member "serviceAccount:service-${PROJECT_NUMBER}@container-engine-robot.iam.gserviceaccount.com"

cat << EOF > trust-config-grpc-xds.yaml
apiVersion: security.cloud.google.com/v1
kind: TrustConfig
metadata:
  name: default
spec:
  trustStores:
  - trustDomain: ${PROJECT_ID}.svc.id.goog
    trustAnchors:
    - certificateAuthorityServiceURI: //privateca.googleapis.com/projects/${PROJECT_ID}/locations/${CAS_ROOT_LOCATION}/caPools/grpc-xds-root
EOF
```

Create a subordinate CA in each region:

```shell
for region in "${REGIONS[@]}"; do
  gcloud privateca pools create grpc-xds-subordinate \
    --location "$region" \
    --tier devops
  
  gcloud privateca subordinates create grpc-xds \
    --auto-enable \
    --pool grpc-xds-subordinate \
    --location "$region" \
    --issuer-pool grpc-xds-root \
    --issuer-location "$CAS_ROOT_LOCATION" \
    --subject "CN=grpc-xds-subordinate-ca, O=Example LLC" \
    --key-algorithm ec-p256-sha256 \
    --use-preset-profile subordinate_mtls_pathlen_0
  
  gcloud privateca pools add-iam-policy-binding grpc-xds-subordinate \
    --location "$region" \
    --role roles/privateca.certificateManager \
    --member "serviceAccount:service-${PROJECT_NUMBER}@container-engine-robot.iam.gserviceaccount.com"
  
  context="gke_${PROJECT_ID}_${region}_grpc-xds"
  kubectl apply --filename trust-config-grpc-xds.yaml --context "$context"

  cat << EOF > "workload-certificate-config-grpc-xds-$region.yaml"
apiVersion: security.cloud.google.com/v1
kind: WorkloadCertificateConfig
metadata:
  name: default
spec:
  certificateAuthorityConfig:
    certificateAuthorityServiceConfig:
      endpointURI: //privateca.googleapis.com/projects/${PROJECT_ID}/locations/${region}/caPools/grpc-xds-subordinate
  keyAlgorithm:
    ecdsa:
      curve: P256
  validityDurationSeconds: 86400
  rotationWindowPercentage: 50
EOF

  kubectl apply --filename "workload-certificate-config-grpc-xds-$region.yaml" --context "$context"
done
```

## Clean up

0. Set up environment variables:

    ```shell
    PROJECT_ID="$(gcloud config get project 2> /dev/null)"
    REGIONS=(us-central1 us-west1)
    AR_LOCATION="${REGIONS[@]:0:1}"
    CAS_ROOT_LOCATION="${REGIONS[@]:0:1}"
    ```

1.  Delete the GKE clusters:

    ```shell
    for region in "${REGIONS[@]}"; do
      gcloud container clusters delete grpc-xds \
        --location "$region" --quiet --async
    done
    ```

2.  Delete the IAM service account:

    ```shell
    gcloud iam service-accounts delete "${GSA}@${PROJECT_ID}.iam.gserviceaccount.com"
    ```

3.  Delete the container image repository in Artifact Registry:

    ```shell
    gcloud artifacts repositories delete grpc-xds \
      --location "$AR_LOCATION" --async --quiet
    ```

4.  Delete the CA Service resources:

    ```shell
    for region in "${REGIONS[@]}"; do
      gcloud privateca subordinates disable grpc-xds \
        --location "$region" --pool grpc-xds-subordinate --quiet
  
      gcloud privateca subordinates delete grpc-xds \
        --location "$region" --pool grpc-xds-subordinate \
        --ignore-active-certificates --skip-grace-period --quiet
  
      gcloud privateca pools delete grpc-xds-subordinate \
        --location "$region" --quiet
      done

    gcloud privateca roots disable grpc-xds \
      --location "$CAS_ROOT_LOCATION" --pool grpc-xds-root --quiet

    gcloud privateca roots delete grpc-xds \
      --location "$CAS_ROOT_LOCATION" --pool grpc-xds-root \
      --ignore-active-certificates --skip-grace-period --quiet

    gcloud privateca pools delete grpc-xds-root \
      --location "$CAS_ROOT_LOCATION" --quiet
    ```

5.  Delete the Cloud NAT resources:

    ```shell
    for region in "${REGIONS[@]}"; do
      gcloud compute routers nats delete grpc-xds --region "$region" \
        --router grpc-xds --router-region "$region" --quiet

      gcloud compute routers delete grpc-xds \
        --region "$region" --quiet
    done
    ```
