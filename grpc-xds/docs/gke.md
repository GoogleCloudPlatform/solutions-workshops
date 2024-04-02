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

3.  Install additional tools that you will use to deploy the xDS control plane
    management service, and the sample gRPC application:

    ```shell
    gcloud components install gke-gcloud-auth-plugin kubectl kustomize skaffold --quiet
    ```
 
4.  Enable the Artifact Registry, GKE, Cloud DNS, and CA Service APIs:

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

0.  Define the region where you will create the Cloud Router:

    ```shell
    REGION=us-central1
    ```

    Note the following about the environment variable:

    - `REGION`: the
      [Compute Engine region](https://cloud.google.com/compute/docs/regions-zones),
      that contains the zones to be used for the GKE cluster
      [node pools](https://cloud.google.com/kubernetes-engine/docs/concepts/node-pools).

1.  Create a Cloud Router:

    ```shell
    gcloud compute routers create grpc-xds \
      --network default \
      --region "$REGION"
    ```

2.  Create a Cloud NAT gateway:

    ```shell
    gcloud compute routers nats create grpc-xds \
      --router grpc-xds \
      --region "$REGION" \
      --auto-allocate-nat-external-ips \
      --nat-all-subnet-ip-ranges
    ```

## Artifact Registry setup

0.  Define environment variables that you use when creating the Artifact
    Registry container image repository:

    ```shell
    LOCATION=us-central1
    PROJECT_ID="$(gcloud config get project 2> /dev/null)"
    PROJECT_NUMBER="$(gcloud projects describe $PROJECT_ID --format 'value(projectNumber)')"
    ```

    Note the following about the environment variables:

    - `LOCATION`: an
      [Artifact Registry location](https://cloud.google.com/artifact-registry/docs/repositories/repo-locations),
      for instance `us-central1`.
      You can use a different location if you like.
    - `PROJECT_ID`: the project ID of your
      [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    - `PROJECT_NUMBER`: the automatically generate project number of your
      [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).

1.  Create a container image repository called `grpc-xds` in Artifact Registry:

    ```shell
    gcloud artifacts repositories create grpc-xds \
      --location "$LOCATION" \
      --repository-format docker
    ```

2.  Configure authentication for `gcloud` and other command-line tools to the
    Artifact Registry host of your repository location:

    ```shell
    gcloud auth configure-docker "${LOCATION}-docker.pkg.dev"
    ```

3.  Grant the
    [Artifact Registry Reader role](https://cloud.google.com/artifact-registry/docs/access-control#roles)
    on the container image repository to the
    [Compute Engine default service account](https://cloud.google.com/compute/docs/access/service-accounts#default_service_account):

    ```shell
    gcloud artifacts repositories add-iam-policy-binding grpc-xds \
      --location "$LOCATION" \
      --member "serviceAccount:${PROJECT_NUMBER}-compute@developer.gserviceaccount.com" \
      --role roles/artifactregistry.reader
    ```

## Creating the Google Kubernetes Engine (GKE) cluster

0.  Define environment variables that you use when creating the GKE clusters:

    ```shell
    ZONES=(us-central1-a us-central1-f)
    PROJECT_ID="$(gcloud config get project 2> /dev/null)"
    ```

    Note the following about the environment variables:

    - `ZONES`: the
      [Compute Engine zones](https://cloud.google.com/compute/docs/regions-zones),
      to be used for the GKE cluster control planes and the default
      [node pools](https://cloud.google.com/kubernetes-engine/docs/concepts/node-pools).
      In the next step you create a GKE cluster for each of the zones in this
      array. You can use different zones if you like.
    - `PROJECT_ID`: the project ID of your
      [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).

1.  Create GKE clusters with zonal control planes and default node pools:

    ```shell
    iter=1
    for zone in "${ZONES[@]}"; do
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
        --location "$zone" \
        --master-ipv4-cidr "172.16.${iter}.64/28" \
        --network default \
        --release-channel rapid \
        --subnetwork default \
        --workload-pool "${PROJECT_ID}.svc.id.goog" \
        --enable-autoscaling \
        --max-nodes 5 \
        --min-nodes 3 \
        --scopes cloud-platform,userinfo-email \
        --tags allow-health-checks,grpc-xds-node \
        --workload-metadata GKE_METADATA
      kubectl config set-context --current --namespace=xds
      iter=$(expr $iter + 1)
    done
    kubectl config use-context "gke_${PROJECT_ID}_${ZONES[1]}_grpc-xds"
    ```

2.  Optional: If you want to create firewalls that only allows access to the
    cluster API servers from your current public IP address:

    ```shell
    public_ip="$(dig TXT +short o-o.myaddr.l.google.com @ns1.google.com | sed 's/"//g')"
    for zone in "${ZONES[@]}"; do
      gcloud container clusters update grpc-xds \
        --enable-master-authorized-networks \
        --location "$zone" \
        --master-authorized-networks "${public_ip}/32"
    done
    ```

## Workload identity federation for GKE

To enable the xDS control plane to communicate with another GKE cluster
control plane, use
[workload identity federation for GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
to enable the xDS control plane Kubernetes service account to authenticate as
an IAM service account without storing long-lived credentials.

0.  Define the name of the IAM service account as an environment variable:

    ```shell
    GSA=xds-control-plane
    ```

    You can use a different name if you like.

1.  Create the IAM service account:

    ```shell
    gcloud iam service-accounts create $GSA
    ```

2.  Grant the
    [Workload Identity User role](https://cloud.google.com/iam/docs/understanding-roles#iam.workloadIdentityUser)
    on the IAM service account to the `control-plane` Kubernetes service
    account in the `xds` namespace:

    ```shell
    gcloud iam service-accounts add-iam-policy-binding "${GSA}@${PROJECT_ID}.iam.gserviceaccount.com" \
      --member "serviceAccount:${PROJECT_ID}.svc.id.goog[xds/control-plane]" \
      --role roles/iam.workloadIdentityUser
    ```

3.  Create a patch to bind the xDS control plane Kubernetes service account to
    the IAM service account, by adding the workload identity federation for
    GKE annotation:

    ```shell
    echo "$(kubectl annotate --dry-run=client --local --output=yaml \
      --filename=k8s/control-plane/base/service-account.yaml \
      iam.gke.io/gcp-service-account=${GSA}@${PROJECT_ID}.iam.gserviceaccount.com)" \
      > k8s/control-plane/base/patch-gke-workload-identity.yaml
    ```

4.  Create a Kubernetes ClusterRoleBinding manifest that grants a Kubernetes
    ClusterRole named `endpointslices-reader` to the IAM service account. The
    ClusterRole provides permissions to list and get EndpointSlices:

    ```shell
    cat << EOF > k8s/control-plane/base/cluster-role-binding-gcp.yaml
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

5.  Apply the `endpointslices-reader` ClusterRole and
    `control-plane-endpointslices-reader-gcp` ClusterRoleBinding to the GKE
    cluster where the xDS control plane is _not_ deployed:

    ```shell
    kubectl apply --context "gke_${PROJECT_ID}_${ZONES[2]}_grpc-xds" \
      --filename k8s/control-plane/base/cluster-role.yaml
    kubectl apply --context "gke_${PROJECT_ID}_${ZONES[2]}_grpc-xds" \
      --filename k8s/control-plane/base/cluster-role-binding-gcp.yaml
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
kubeconfig_dir=k8s/control-plane/components/kubeconfig
kubeconfig_file="${kubeconfig_dir}/kubeconfig.yaml"
rm -f "$kubeconfig_file"

iter=1
for zone in "${ZONES[@]}"; do
  cluster="grpc-xds-${zone}"
  context="grpc-xds-${iter}"
  context="${context%-1}"

  kubectl config set-context "$context" \
    --cluster "$cluster" \
    --user user \
    --kubeconfig "$kubeconfig_file"

  cluster_ca_file="$(mktemp)"
  gcloud container clusters describe grpc-xds --format='value(masterAuth.clusterCaCertificate)' --location=$zone | base64 --decode > "$cluster_ca_file"
  kubectl config set-cluster "$cluster" \
    --certificate-authority "$cluster_ca_file" \
    --embed-certs \
    --server "https://$(gcloud container clusters describe grpc-xds --format='value(privateClusterConfig.privateEndpoint)' --location=$zone)" \
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
certificates to pods on the GKE clusters.

Create the `WorkloadCertificateConfig` and `TrustConfig` resources in both of
the GKE clusters.

The commands are listed below:

```shell
LOCATION=us-central1
PROJECT_ID="$(gcloud config get project 2> /dev/null)"
PROJECT_NUMBER="$(gcloud projects describe $PROJECT_ID --format 'value(projectNumber)')"

gcloud privateca pools create grpc-xds-root-ca-pool \
  --location "$LOCATION" \
  --tier enterprise

gcloud privateca roots create grpc-xds-root-ca \
  --auto-enable \
  --pool grpc-xds-root-ca-pool \
  --subject "CN=grpc-xds-root-ca, O=Example LLC" \
  --key-algorithm ec-p256-sha256 \
  --max-chain-length 1 \
  --location "$LOCATION"

gcloud privateca pools create grpc-xds-subordinate-ca-pool \
  --location "$LOCATION" \
  --tier devops

gcloud privateca subordinates create grpc-xds-subordinate-ca \
  --auto-enable \
  --pool grpc-xds-subordinate-ca-pool \
  --location "$LOCATION" \
  --issuer-pool grpc-xds-root-ca-pool \
  --issuer-location "$LOCATION" \
  --subject "CN=grpc-xds-subordinate-ca, O=Example LLC" \
  --key-algorithm ec-p256-sha256 \
  --use-preset-profile subordinate_mtls_pathlen_0

gcloud privateca pools add-iam-policy-binding grpc-xds-root-ca-pool \
  --location "$LOCATION" \
  --role roles/privateca.auditor \
  --member "serviceAccount:service-${PROJECT_NUMBER}@container-engine-robot.iam.gserviceaccount.com"

gcloud privateca pools add-iam-policy-binding grpc-xds-subordinate-ca-pool \
  --location "$LOCATION" \
  --role roles/privateca.certificateManager \
  --member "serviceAccount:service-${PROJECT_NUMBER}@container-engine-robot.iam.gserviceaccount.com"

cat << EOF > WorkloadCertificateConfig.yaml
apiVersion: security.cloud.google.com/v1
kind: WorkloadCertificateConfig
metadata:
  name: default
spec:
  certificateAuthorityConfig:
    certificateAuthorityServiceConfig:
      endpointURI: //privateca.googleapis.com/projects/${PROJECT_ID}/locations/${LOCATION}/caPools/grpc-xds-subordinate-ca-pool
  keyAlgorithm:
    ecdsa:
      curve: P256
  validityDurationSeconds: 86400
  rotationWindowPercentage: 50
EOF

cat << EOF > TrustConfig.yaml
apiVersion: security.cloud.google.com/v1
kind: TrustConfig
metadata:
  name: default
spec:
  trustStores:
  - trustDomain: ${PROJECT_ID}.svc.id.goog
    trustAnchors:
    - certificateAuthorityServiceURI: //privateca.googleapis.com/projects/${PROJECT_ID}/locations/${LOCATION}/caPools/grpc-xds-root-ca-pool
EOF

for zone in "${ZONES[@]}"; do
  context="gke_${PROJECT_ID}_${zone}_grpc-xds"
  kubectl apply --filename WorkloadCertificateConfig.yaml --context "$context"
  kubectl apply --filename TrustConfig.yaml --context "$context"
done
```

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

1.  Delete the GKE clusters:

    ```shell
    for zone in "${ZONES[@]}"; do
      gcloud container clusters delete grpc-xds \
        --location "$zone" --quiet --async
    done
    ```

2.  Delete the IAM service account:

    ```shell
    gcloud iam service-accounts delete "${GSA}@${PROJECT_ID}.iam.gserviceaccount.com"
    ```

3.  Delete the container image repository in Artifact Registry:

    ```shell
    gcloud artifacts repositories delete grpc-xds \
      --location "$LOCATION" --async --quiet
    ```

4.  Delete the CA Service resources:

    ```shell
    gcloud privateca subordinates disable grpc-xds-subordinate-ca \
      --location "$LOCATION" --pool grpc-xds-subordinate-ca-pool --quiet

    gcloud privateca subordinates delete grpc-xds-subordinate-ca \
      --location "$LOCATION" --pool grpc-xds-subordinate-ca-pool \
      --ignore-active-certificates --skip-grace-period --quiet

    gcloud privateca pools delete grpc-xds-subordinate-ca-pool \
      --location "$LOCATION" --quiet

    gcloud privateca roots disable grpc-xds-root-ca \
      --location "$LOCATION" --pool grpc-xds-root-ca-pool --quiet

    gcloud privateca roots delete grpc-xds-root-ca \
      --location "$LOCATION" --pool grpc-xds-root-ca-pool \
      --ignore-active-certificates --skip-grace-period --quiet

    gcloud privateca pools delete grpc-xds-root-ca-pool \
      --location "$LOCATION" --quiet
    ```

5.  Delete the Cloud NAT resources:

    ```shell
    gcloud compute routers nats delete grpc-xds \
      --router grpc-xds --region "$REGION" --quiet

    gcloud compute routers delete grpc-xds \
      --region "$REGION" --quiet
    ```
