# grpc-xds-workshop

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
plane and the sample gRPC application on either a
[Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/docs)
cluster, or on a local [kind](https://kind.sigs.k8s.io/) Kubernetes cluster.

The code in this repository in not recommended for production environments, and
there are no plans to make it production-ready. Instead, we recommend
[Traffic Director](https://cloud.google.com/traffic-director/docs) from Google Cloud.

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

The [`k8s`](k8s) directory contains Kubernetes manifests and patches that are
used by both the Go and Java implementations of the control plane and the
sample application. Patches that are specific to the Go and Java
implementations can be found in `k8s` subdirectories of
`control-plane-(go|java)` and `greeter-(go|java)`. Skaffold renders the
complete manifests using Kustomize.

The [`k8s/troubleshoot`](k8s/troubleshoot) directory contains Kubernetes
manifests that you can use to deploy a pod with networking and gRPC
troubleshooting tools to your cluster.

## Prerequisites

To run the samples, you need a Kubernetes cluster. The sample works best with
[Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/docs)
and [Artifact Registry](https://cloud.google.com/artifact-registry/docs),
but you can also use other clusters and container image registries, including
local clusters using [kind](https://kind.sigs.k8s.io/).

The [`docs`](docs) directory contains instructions on both
[creating a GKE cluster and Artifact Registry repository](docs/gke.md), or
creating a multi-node Kubernetes cluster using [kind](docs/kind.md), and on
creating certificate authorities (CAs) for issuing workload TLS certificates.

If you want to build and deploy the Java control plane and sample application,
you need [Java 17](https://adoptium.net/).

If you want to build and deploy the Go control plane and sample application,
you need [Go 1.20](https://go.dev/doc/install) or later.

You also need the following tools:

- [kubectl](https://kubernetes.io/docs/reference/kubectl/)
- [Kustomize](https://kustomize.io/)
- [Skaffold](https://skaffold.dev/)
- [gRPCurl](https://github.com/fullstorydev/grpcurl)

If you have already installed the
[Google Cloud SDK](https://cloud.google.com/sdk/docs/install),
you can install kubectl, Kustomize, and Skaffold using the `gcloud` command:

```shell
gcloud components install kubectl kustomize skaffold
```

If you use macOS, you can use `brew` to install kubectl, Kustomize, Skaffold,
and gRPCurl:

```shell
brew install kubectl kustomize skaffold grpcurl
```

Follow the steps in the documents
[Verify local development setup using Go](docs/verify-local-setup-go.md) or
[Verify local development setup using Java](docs/verify-local-setup-java.md)
to ensure that your cluster and the prerequisite tools are set up correctly.

## Local Kubernetes cluster setup using kind

Follow the the documentation on
[create a multi-node kind Kubernetes cluster](docs/kind.md) with fake
[zone labels (`topology.kubernetes.io/zone`)](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone),
and set up a root certificate authority using
[cert-manager](https://cert-manager.io/docs/)
to issue workload TLS certificates.

## GKE cluster and Artifact Registry setup

1.  Follow the documentation on
    [creating a GKE cluster, an Artifact Registry container image repository, and certificate authorities for issuing workload TLS certificates](docs/gke.md).

2.  Create and export an environment variable called `SKAFFOLD_DEFAULT_REPO`
    to point to your container image registry:

    ```shell
    export SKAFFOLD_DEFAULT_REPO=LOCATION-docker.pkg.dev/PROJECT_ID/REPOSITORY
    ```

    Replace the following:

    - `LOCATION`: the
      [location](https://cloud.google.com/artifact-registry/docs/repositories/repo-locations)
      of your Artifact Registry container image repository.
    - `PROJECT_ID`: your Google Cloud
      [project ID](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    - `REPOSITORY`: the name of your Artifact Registry
      [container image repository](https://cloud.google.com/artifact-registry/docs/docker).

## Running the xDS control plane and sample gRPC application

1.  Build the container images for the control plane and the sample gRPC
    application, render the Kubernetes resource manifests, apply them to the
    Kubernetes cluster, and set up port forwarding.

    Using the Go implementations:

    ```shell
    make run-go
    ```

    Using the Java implementations:

    ```shell
    make run-java
    ```

    Leave Skaffold running so that port forwarding keeps working.

2.  In a new terminal, tail the control plane logs:

    ```shell
    make tail-control-plane
    ```

3.  In another new terminal, tail the greeter-intermediary logs:

    ```shell
    make tail-greeter-intermediary
    ```

4.  In yet another new terminal, tail the greeter-leaf logs:

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

6.  Send a request to the `greeter-leaf` server, using mTLS and the
    [DNS resolver](https://grpc.io/docs/guides/custom-name-resolution/):

    ```shell
    grpcurl \
      -authority greeter-leaf \
      -cacert ./certs/ca_certificates.pem \
      -cert ./certs/certificates.pem \
      -key ./certs/private_key.pem \
      -d '{"name": "World"}' \
      dns:///localhost:50057 \
      helloworld.Greeter/SayHello
    ```

    Or, using a target from the included [`Makefile`](Makefile):

    ```shell
    make request-leaf-mtls
    ```

7.  Send a request to the `greeter-intermediary` server, using mTLS and the
    [DNS resolver](https://grpc.io/docs/guides/custom-name-resolution/):

    ```shell
    grpcurl \
      -authority greeter-intermediary \
      -cacert ./certs/ca_certificates.pem \
      -cert ./certs/certificates.pem \
      -key ./certs/private_key.pem \
      -d '{"name": "World"}' \
      dns:///localhost:50055 \
      helloworld.Greeter/SayHello
    ```

    Or, using a target from the included [`Makefile`](Makefile):

    ```shell
    make request-mtls
    ```

8.  Observe the control plane logs as you scale the greeter service
    deployments. For instance, you can scale the `greeter-leaf` deployment:

    ```shell
    kubectl scale deployment/greeter-leaf --namespace=xds --replicas=2
    ```

9.  To explore the cluster resources, you may find it convenient to set the
    namespace for your current kubeconfig context entry:

    ```shell
    kubectl config set-context --current --namespace=xds
    ```

10. To delete the control plane and greeter pods, without deleting the `xds`
    namespace or other resources, such as `cert-manager`:

    ```shell
    make delete
    ```

See the [`Makefile`](Makefile) for examples of other commands you can run.

## Development

Set up a local file watch that automatically rebuilds and redeploys the
applications on code changes:

Using Go:

```shell
make dev-go
```

Using Java:

```shell
make dev-java
```

## Remote debugging

Set up remote debugging by exposing and port-forwarding to delve (for Go) or
the JDWP agent (for Java):

Using Go:

```shell
make debug-go
```

Using Java:

```shell
make debug-java
 ```

## Troubleshooting

1.  Create a pod in the Kubernetes cluster with various tools available to
    troubleshoot issues.

    ```shell
    make run-bastion
    ```

    This takes a few minutes, as an init container installs a number of tools.

2.  Open an interactive shell in the pod's container:

    ```shell
    make troubleshoot
    ```

Some troubleshooting commands:

- View the Kubernetes pod IP addresses:

  ```shell
  kubectl get pods --namespace=xds --output=wide
  ```

- View the operating xDS configuration of a server using
  [Client Status Discovery Service (CSDS)](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/status/v3/csds.proto):

  ```shell
  grpcurl -plaintext POD_IP:50052 envoy.service.status.v3.ClientStatusDiscoveryService/FetchClientStatus | yq --prettyPrint
  ```

  Replace `POD_IP` with the IP address of the Kubernetes pod.

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

- Send a request to the `greeter-leaf` service using mTLS and xDS:

  ```shell
  grpcurl \
    -authority greeter-leaf \
    -cacert /var/run/secrets/workload-spiffe-credentials/ca_certificates.pem \
    -cert /var/run/secrets/workload-spiffe-credentials/certificates.pem \
    -key /var/run/secrets/workload-spiffe-credentials/private_key.pem \
    -d '{"name": "World"}' \
    xds:///greeter-leaf \
    helloworld.Greeter/SayHello
  ```

  Set the `GRPC_GO_LOG_SEVERITY_LEVEL` and `GRPC_GO_LOG_VERBOSITY_LEVEL`
  environment variables to see addtional log messages from `grpcurl`'s
  interaction with the xDS control plane management server:

  ```shell
  export GRPC_GO_LOG_SEVERITY_LEVEL=info
  export GRPC_GO_LOG_VERBOSITY_LEVEL=99
  ```

- Send a request to the `greeter-intermediary` service using mTLS and xDS:

  ```shell
  grpcurl \
    -authority greeter-intermediary \
    -cacert /var/run/secrets/workload-spiffe-credentials/ca_certificates.pem \
    -cert /var/run/secrets/workload-spiffe-credentials/certificates.pem \
    -key /var/run/secrets/workload-spiffe-credentials/private_key.pem \
    -d '{"name": "World"}' \
    -import-path /opt/protos -proto helloworld/greeter.proto -proto google/rpc/error_details.proto \
    xds:///greeter-intermediary \
    helloworld.Greeter/SayHello
  ```

- View the xDS bootstrap configuration file of the troubleshooting pod:

  ```shell
  cat $GRPC_XDS_BOOTSTRAP
  ```

## Cleaning up

Delete all the deployed resources in the Kubernetes cluster:

```shell
make clean
```

## Kubernetes references

- [API Concepts](https://kubernetes.io/docs/reference/using-api/api-concepts/)
- [Headless Services](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services)
- [EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)

## xDS references

- [xDS API Overview](https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/overview)
- [xDS REST and gRPC protocol](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [Aggregated Discovery Service (ADS)](https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/xds_api#aggregated-discovery-service)
- [gRFC A27: xDS-Based Global Load Balancing](https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md)
- [gRFC A29: xDS-Based Security for gRPC Clients and Servers](https://github.com/grpc/proposal/blob/deaf1bcf248d1e48e83c470b00930cbd363fab6d/A29-xds-tls-security.md)
- [gRFC A36: xDS-Enabled Servers](https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md)
- [xDS Features in gRPC](https://github.com/grpc/grpc/blob/1b31c6e0ba711787c05e8e78719896a682fca102/doc/grpc_xds_features.md)

## Disclaimer

This is not an officially supported Google product.
