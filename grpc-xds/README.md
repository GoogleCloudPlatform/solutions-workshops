# grpc-xds

This directory contains sample Go and Java implementations of
[xDS](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/dynamic_configuration)
control planes for gRPC services running on Kubernetes.

The goal of the control plane implementations is to provide a practical
understanding of xDS, beyond the API specifications and the
[pithy](https://github.com/grpc/grpc-go/tree/v1.59.0/examples/features/xds)
[examples](https://github.com/grpc/grpc-java/tree/v1.59.0/examples/example-xds).

This directory also contains accompanying sample applications implemented in
Go and Java. The sample applications implement the
[`helloworld.Greeter`](https://github.com/grpc/grpc-go/blob/v1.59.0/examples/helloworld/helloworld/helloworld.proto)
gRPC service.

The scripts and manifests in this directory enable running the xDS control
plane and the sample gRPC application on either
[Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/docs)
clusters, or on local [kind](https://kind.sigs.k8s.io/) Kubernetes clusters.

The code in this repository in not recommended for production environments.
If you want a production-ready xDS control plane, we recommend
[Cloud Service Mesh](https://cloud.google.com/service-mesh/docs/service-routing/proxyless-overview)
from Google Cloud.

## Directory structure

The  [`control-plane-go`](control-plane-go) and
[`control-plane-java`](control-plane-java) directories contain the sample xDS
control plane management server implementations.

The [`greeter-go`](greeter-go) and [`greeter-java`](greeter-java) directories
contain Go and Java implementations of sample gRPC servers that implement the
[`helloworld.Greeter` service](https://grpc.io/docs/languages/go/quickstart/#update-the-grpc-service).
Each implementation can be deployed either as a "leaf" service, or as an
"intermediary" service.

The leaf service (`greeter-leaf`) responds with its host name and the cloud
service provider zone of the underlying compute node.

The intermediary service (`greeter-intermediary`) forwards the request to the
next hop, which is another `helloworld.Greeter` service implementation. Upon
receiving a response from the next hop (defined by the `NEXT_HOP` environment
variable), the intermediary service appends its own host name and zone, before
returning the response to the client.

The [`hack`](hack) directory contains shell scripts for deploying to multiple
Kubernetes clusters.

The [`k8s`](k8s) directory contains Kubernetes manifests and patches that are
used by both the Go and Java implementations of the xDS control plane and the
sample gRPC applications. Patches that are specific to the Go and Java
implementations can be found in `k8s` subdirectories of
`control-plane-(go|java)` and `greeter-(go|java)`. Skaffold renders the
complete manifests using Kustomize.

The [`k8s/troubleshoot`](k8s/troubleshoot) directory contains Kubernetes
manifests that you can use to deploy a Pod with networking and gRPC
troubleshooting tools to your cluster.

## Prerequisites

To run the samples, you need one or more Kubernetes clusters. The sample works
great with
[Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/docs)
and [Artifact Registry](https://cloud.google.com/artifact-registry/docs),
but you can also use local clusters using [kind](https://kind.sigs.k8s.io/).

The [`docs`](docs) directory contains instructions on both
[creating GKE clusters and an Artifact Registry container image repository](docs/gke.md),
or creating multi-node Kubernetes clusters using [kind](docs/kind.md), and on
creating certificate authorities (CAs) for issuing workload TLS certificates.

If you want to build and deploy the Java implementation of the xDS control
plane and sample gRPC application, you need [Java 17](https://adoptium.net/).

If you want to build and deploy the Go implementation of the xDS control plane
and sample gRPC applications, you need [Go 1.22](https://go.dev/doc/install)
or later.

You also need the following tools:

- [kubectl](https://kubernetes.io/docs/reference/kubectl/) at a version that is
  [supported by the Kubernetes control plane version that you use (`kube-apiserver` specifically)](https://kubernetes.io/releases/version-skew-policy/#kubectl)
- [Kustomize](https://kustomize.io/) v4.5.5 or later
- [Skaffold](https://skaffold.dev/) v2.10.1 or later
- [gRPCurl](https://github.com/fullstorydev/grpcurl) v1.9.1 or later
- [yq](https://mikefarah.gitbook.io/yq/) v4.41.1 or later

To make it easier to switch between kubeconfig contexts for multiple
Kubernetes clusters, you can also optionally install
[`kubectx`](https://github.com/ahmetb/kubectx).

If you use macOS, you can use `brew` to install kubectl, Kustomize, Skaffold,
gRPCurl, yq, kind, and kubectx:

```shell
brew update
brew install kubectl kustomize skaffold grpcurl yq kind kubectx
```

Follow the steps in the documents
[Verify local development setup using Go](docs/verify-local-setup-go.md) or
[Verify local development setup using Java](docs/verify-local-setup-java.md)
to ensure that your cluster and the prerequisite tools are set up correctly.

## Local Kubernetes cluster setup using kind

Follow the the documentation on
[create multi-node kind Kubernetes clusters](docs/kind.md) with fake
[zone labels (`topology.kubernetes.io/zone`)](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone),
and set up a root certificate authority using
[cert-manager](https://cert-manager.io/docs/)
to issue workload TLS certificates.

## GKE and Artifact Registry setup

1.  Follow the documentation on
    [creating GKE clusters, an Artifact Registry container image repository, and certificate authorities in Certificate Authority Service (CA Service) for issuing workload TLS certificates](docs/gke.md).

2.  Create and export an environment variable called `SKAFFOLD_DEFAULT_REPO`
    to point to your container image registry:

    ```shell
    export SKAFFOLD_DEFAULT_REPO=$AR_LOCATION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY
    ```

    Replace the following:

    - `$AR_LOCATION`: the
      [location](https://cloud.google.com/artifact-registry/docs/repositories/repo-locations)
      of your Artifact Registry container image repository, e.g., `us-central1` or `us`.
    - `$PROJECT_ID`: your Google Cloud
      [project ID](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    - `$REPOSITORY`: the name of your Artifact Registry
      [container image repository](https://cloud.google.com/artifact-registry/docs/docker),
      e.g., `grpc-xds`.

## Running the xDS control plane and sample gRPC applications

1.  Build the container images for the xDS control plane and the sample gRPC
    applications, render the Kubernetes resource manifests, apply them to the
    Kubernetes cluster, and set up port forwarding.

    Leave Skaffold running so that port forwarding keeps working.

    Choose one of the run options below, and choose whether you want to deploy
    the Go (`go`) or Java (`java`) implementations.

    1.  Run the xDS control plane and the sample applications
        `greeter-intermediary` and `greeter-leaf` on one Kubernetes cluster:

        ```shell
        make run-[go|java]
        ```

    2.  Run the sample applications `greeter-intermediary` and
        `greeter-leaf` on two Kubernetes clusters, and the xDS control plane
        on one of the Kubernetes clusters:

        ```shell
        make run-[go|java]-multi-cluster
        ```

        In this configuration, the sample applications running on the two
        Kubernetes clusters all connect to the same xDS control plane.

    3.  Run the xDS control plane and the sample applications
        `greeter-intermediary` and `greeter-leaf` on two Kubernetes cluster,
        with xDS federation enabled:

        ```shell
        make run-[go|java]-federation
        ```

        In this configuration, the sample applications receive xDS
        configuration from the xDS control plane in their respective
        Kubernetes clusters.

        The `greeter-intermediary` sample application running on one of the
        Kubernetes clusters sets up a gRPC channel to the `greeter-leaf`
        sample application running on the other Kubernetes cluster, by
        retrieving xDS resources from the xDS control plane running on the
        other Kubernetes cluster.

    4.  Run the sample applications `greeter-intermediary` and `greeter-leaf`
        in one Kubernetes cluster _without_ using xDS:

        ```shell
        cd greeter-[go|java]
        make run-no-xds
        ```

        In this configuration, the `greeter-intermediary` sample application
        discovers and load balances connections to the `greeter-leaf` sample
        application using Kubernetes cluster DNS and cluster IP.

    You may see messages similar to the following during deployment:

    `xds:deployment/greeter-leaf: FailedMount: MountVolume.SetUp failed for volume "workload-certs" : secret "greeter-leaf-cert" not found`

    You can safely ignore these messages as long as the deployment proceeds,
    and you eventually see the message `Deployments stabilized in __ seconds`.

2.  In a new terminal, tail the xDS control plane logs:

    ```shell
    make tail-control-plane
    ```

3.  In another new terminal, tail the `greeter-intermediary` logs from the
    Kubernetes cluster referenced by your current kubeconfig context:

    ```shell
    make tail-greeter-intermediary
    ```

4.  In yet another new terminal, tail the `greeter-leaf` logs from the
    Kubernetes cluster referenced by your current kubeconfig context:

    ```shell
    make tail-greeter-leaf
    ```

5.  In - you guessed it - a new terminal, create a private key and TLS
    certificate for your developer workstation. You do this by creating a
    temporary Kubernetes Pod with a workload TLS certificate and private key,
    copy this certificate and private key to your developer workstation, and
    deleting the temporary Pod:

    ```shell
    make host-certs
    ```

    The workload certificate, CA certificate, and private key will be copied
    to the `certs` directory.

6.  Send a request to the `greeter-leaf` server in one of the clusters, using
    mTLS and the
    [DNS resolver](https://grpc.io/docs/guides/custom-name-resolution/):

    ```shell
    make request-leaf-mtls
    ```

    If you use GKE workload TLS certificates, bypass certificate verification,
    as gRPCurl cannot verify the
    [SPIFFE ID](https://spiffe.io/docs/latest/spiffe-about/spiffe-concepts/#spiffe-id)
    in the gRPC server's workload TLS certificates:

    ```shell
    make request-leaf-mtls-noverify
    ```

7.  Send a request to the `greeter-intermediary` server in one of the
    clusters, using mTLS and the
    [DNS resolver](https://grpc.io/docs/guides/custom-name-resolution/):

    ```shell
    make request-mtls
    ```

    If you use GKE workload TLS certificates, bypass certificate verification,
    as gRPCurl cannot verify the
    [SPIFFE ID](https://spiffe.io/docs/latest/spiffe-about/spiffe-concepts/#spiffe-id)
    in the gRPC server's workload TLS certificates:

    ```shell
    make request-mtls-noverify
    ```

8.  Observe the xDS control plane logs as you scale the greeter Deployments.
    For instance, you can scale the `greeter-leaf` Deployment in one of the
    clusters:

    ```shell
    kubectl scale deployment/greeter-leaf --namespace=xds --replicas=2
    ```

9.  To explore resources across both clusters, use either `kubectx`, or the
    `kubectl config current-context`, `kubectl config get-contexts`, and
    `kubectl config use-context` commands.

10. Delete the xDS control plane and `greeter` Deployment, Service, and
    ConfigMap resources in both Kubernetes clusters:

    ```shell
    make delete-apps
    ```

    This command does _not_ delete the `xds` namespaces or other resources,
    such as the GKE workload TLS certificate configuration resources,
    `cert-manager`, or the bastion Pod.

See the [`Makefile`](Makefile) for examples of other commands you can run.

## Development

Deploy to one cluster only, and set up a local file watch that automatically
rebuilds and redeploys the xDS control plane and gRPC applications on code
changes:

```shell
make dev-[go|java]
```

## Remote debugging

Deploy to one cluster only, and set up remote debugging by exposing and
port-forwarding to `delve` (for Go) or the JDWP agent (for Java):

```shell
make debug-[go|java]
```

## Troubleshooting

1.  Create a bastion Pod in one of the Kubernetes clusters with various tools
    available to troubleshoot issues.

    ```shell
    make run-bastion
    ```

    This takes a few minutes, as an init container installs a number of tools.

    The bastion Pod is configured to only have access to the API server of the
    Kubernetes cluster where the Pod is deployed. To troubleshoot two
    clusters, create a bastion Pod in each cluster.

2.  Open an interactive shell in the Pod's container:

    ```shell
    make troubleshoot
    ```

Some troubleshooting commands:

- View the Kubernetes Pod IP addresses:

  ```shell
  kubectl get pods --namespace=xds --output=wide
  ```

- View the operating xDS configuration of an xDS-enabled gRPC server using
  [Client Status Discovery Service (CSDS)](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/status/v3/csds.proto):

  ```shell
  grpcurl -plaintext POD_IP:50052 envoy.service.status.v3.ClientStatusDiscoveryService/FetchClientStatus | yq --prettyPrint
  ```

  Replace `POD_IP` with the IP address of the Kubernetes Pod.

- List the ACK'ed xDS resources of a gRPC server using `grpcdebug`:

  ```shell
  grpcdebug POD_IP:50052 xds status
  ```

- View the operating xDS configuration of a gRPC server using `grpcdebug`:

  ```shell
  grpcdebug POD_IP:50052 xds config | yq --prettyPrint
  ```

- View the Listener Discovery Service (LDS) configuration only of a gRPC
  server using `grpcdebug`:

  ```shell
  grpcdebug POD_IP:50052 xds config --type LDS | yq --input-format=json --prettyPrint
  ```

  Replace `LDS` with other xDS services to view other ACK'ed xDS resources.

- Set the `GRPC_GO_LOG_SEVERITY_LEVEL` and `GRPC_GO_LOG_VERBOSITY_LEVEL`
  environment variables to see addtional log messages from gRPCurl's
  interaction with the xDS control plane management server:

  ```shell
  export GRPC_GO_LOG_SEVERITY_LEVEL=info
  export GRPC_GO_LOG_VERBOSITY_LEVEL=99
  ```

- Send a request to the `greeter-leaf` service using mTLS and xDS:

  ```shell
  grpcurl \
    -cacert /var/run/secrets/workload-spiffe-credentials/ca_certificates.pem \
    -cert /var/run/secrets/workload-spiffe-credentials/certificates.pem \
    -key /var/run/secrets/workload-spiffe-credentials/private_key.pem \
    -d '{"name": "World"}' \
    -import-path /opt/protos \
    -insecure \
    -proto google/rpc/error_details.proto \
    -proto helloworld/greeter.proto \
    xds:///greeter-leaf \
    helloworld.Greeter/SayHello
  ```

- Send a request to the `greeter-intermediary` service using mTLS and xDS:

  ```shell
  grpcurl \
    -cacert /var/run/secrets/workload-spiffe-credentials/ca_certificates.pem \
    -cert /var/run/secrets/workload-spiffe-credentials/certificates.pem \
    -key /var/run/secrets/workload-spiffe-credentials/private_key.pem \
    -d '{"name": "World"}' \
    -import-path /opt/protos \
    -insecure \
    -proto google/rpc/error_details.proto \
    -proto helloworld/greeter.proto \
    xds:///greeter-intermediary \
    helloworld.Greeter/SayHello
  ```

- View the xDS bootstrap configuration file of the bastion Pod:

  ```shell
  jq < $GRPC_XDS_BOOTSTRAP
  ```

## Kubernetes references

- [API Concepts](https://kubernetes.io/docs/reference/using-api/api-concepts/)
- [Headless Services](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services)
- [EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)
- [Namespace Sameness](https://github.com/kubernetes/community/blob/dd4c8b704ef1c9c3bfd928c6fa9234276d61ad18/sig-multicluster/namespace-sameness-position-statement.md)

## xDS references

- [On the state of Envoy Proxy control planes](https://mattklein123.dev/2020/03/15/2020-03-14-on-the-state-of-envoy-proxy-control-planes/)
- [xDS API Overview](https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/overview)
- [xDS REST and gRPC protocol](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [xDS Features in gRPC](https://github.com/grpc/grpc/blob/master/doc/grpc_xds_features.md)
- [Aggregated Discovery Service (ADS)](https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/xds_api#aggregated-discovery-service)
- [Load Reporting Service (LRS)](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/load_stats/v3/lrs.proto)
- [Open Request Cost Aggregation (ORCA)](https://github.com/envoyproxy/envoy/issues/6614)
- [gRFC A14: gRPC Channelz](https://github.com/grpc/proposal/blob/6657c0723c185bda949babd120cf6e5e701cb91b/A14-channelz.md)
- [gRFC A27: xDS-Based Global Load Balancing](https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md)
- [gRFC A28: gRPC xDS traffic splitting and routing](https://github.com/grpc/proposal/blob/f6d38361da31ffad3158dbef84f8af3cdd89d8c1/A28-xds-traffic-splitting-and-routing.md)
- [gRFC A29: xDS-Based Security for gRPC Clients and Servers](https://github.com/grpc/proposal/blob/deaf1bcf248d1e48e83c470b00930cbd363fab6d/A29-xds-tls-security.md)
- [gRFC A30: xDS v3 Support](https://github.com/grpc/proposal/blob/c7546e5e13f72d67b53f492a73c31184bb11aa06/A30-xds-v3.md)
- [gRFC A31: gRPC xDS Timeout Support and Config Selector Design](https://github.com/grpc/proposal/blob/2d22835c8375856f56cffcbb81ec420bbd1e5132/A31-xds-timeout-support-and-config-selector.md)
- [gRFC A32: gRPC xDS circuit breaking](https://github.com/grpc/proposal/blob/33d7b7d58d51c3fe9fbee6862821efe9d43aa963/A32-xds-circuit-breaking.md)
- [gRFC A33: Client-Side Fault Injection](https://github.com/grpc/proposal/blob/c67f477a3e4f37931db73c0a993bd2859172d8e8/A33-Fault-Injection.md)
- [gRFC A36: xDS-Enabled Servers](https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md)
- [gRFC A37: xDS Aggregate and Logical DNS Clusters](https://github.com/grpc/proposal/blob/7c05212d14f4abef5f74f71695f95ba8dd3f7dd3/A37-xds-aggregate-and-logical-dns-clusters.md)
- [gRFC A39: xDS HTTP Filter Support](https://github.com/grpc/proposal/blob/b2093bc96e045d7a13ccc01886b9dde346c2b83b/A39-xds-http-filters.md)
- [gRFC A40: xDS Configuration Dump via Client Status Discovery Service in gRPC](https://github.com/grpc/proposal/blob/312af83e2f6af1ccb9b048b967635f67c8d40643/A40-csds-support.md)
- [gRFC A41: xDS RBAC Support](https://github.com/grpc/proposal/blob/c83f0cb8ed534c4192e0e5d7a4550a1f5a76ef65/A41-xds-rbac.md)
- [Envoy Role Based Access Control (RBAC) HTTP Filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/rbac_filter)
- [gRFC A42: xDS Ring Hash LB Policy](https://github.com/grpc/proposal/blob/1d50990f5e95d5d15233e35e56ac1dc33fcd56b3/A42-xds-ring-hash-lb-policy.md)
- [gRFC A44: gRPC xDS Retry Support](https://github.com/grpc/proposal/blob/6b550edf7407047d423c8fa530e4f9d2e50a2fd2/A44-xds-retry.md)
- [gRFC A47: xDS Federation](https://github.com/grpc/proposal/blob/e85c66e48348867937688d89117bad3dcaa6f4f5/A47-xds-federation.md)
- [gRFC A48: xDS Least Request LB Policy](https://github.com/grpc/proposal/blob/fc4ade7b042a614aba5df8b6b82f4ca55f59a37e/A48-xds-least-request-lb-policy.md)
- [gRFC A50: gRPC xDS Outlier Detection Support](https://github.com/grpc/proposal/blob/ee75a4010214ddda02ba992e69f1c57be7f71497/A50-xds-outlier-detection.md)
- [gRFC A51: Custom Backend Metrics Support](https://github.com/grpc/proposal/blob/9208d9866293fcf85f2802332f34bdec106717a9/A51-custom-backend-metrics.md)
- [gRFC A52: gRPC xDS Custom Load Balancer Configuration](https://github.com/grpc/proposal/blob/7c05212d14f4abef5f74f71695f95ba8dd3f7dd3/A52-xds-custom-lb-policies.md)
- [gRFC A53: Option for Ignoring xDS Resource Deletion](https://github.com/grpc/proposal/blob/1f9b52226bf45d9f54d0eda34446b4cffabfcec6/A53-xds-ignore-resource-deletion.md)
- [gRFC A55: xDS-Based Stateful Session Affinity for Proxyless gRPC](https://github.com/grpc/proposal/blob/9a2bd577d48b45aa9125e6a49b115690042371fe/A55-xds-stateful-session-affinity.md)
- [gRFC A57: XdsClient Failure Mode Behavior](https://github.com/grpc/proposal/blob/f1ef153e9955d3507e8322727e96e56e04933605/A57-xds-client-failure-mode-behavior.md)
- [gRFC A58: `weighted_round_robin` LB policy](https://github.com/grpc/proposal/blob/0ba0c1905050525f9b0aee46f3f23c8e1e515489/A58-client-side-weighted-round-robin-lb-policy.md)
- [gRFC A65: mTLS Credentials in xDS Bootstrap File](https://github.com/grpc/proposal/blob/e027a56d7d900b47948602e6d72413b5cba80d54/A65-xds-mtls-creds-in-bootstrap.md)
- [xRFC TP1: `xdstp://` structured resource naming, caching and federation support](https://github.com/cncf/xds/blob/70da609f752ed4544772f144411161d41798f07e/proposals/TP1-xds-transport-next.md)
- [xRFC TP2: Dynamically Generated Cacheable xDS Resources](https://github.com/cncf/xds/blob/70da609f752ed4544772f144411161d41798f07e/proposals/TP2-dynamically-generated-cacheable-xds-resources.md)

## Disclaimer

This is not an officially supported Google product.
