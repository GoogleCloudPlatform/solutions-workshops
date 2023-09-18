# grpc-xds-workshop

This directory contains sample Go and Java implementations of
[xDS](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/dynamic_configuration)
control planes for gRPC services running on Kubernetes.

The goal of the control plane implementations is to provide a practical
understanding of xDS, beyond the API specifications and the
[pithy](https://github.com/grpc/grpc-go/tree/v1.58.0/examples/features/xds)
[examples](https://github.com/grpc/grpc-java/tree/v1.58.0/examples/example-xds).

This directory also contains accompanying sample applications implemented in
Go and Java. The sample applications implement the
[`helloworld.Greeter`](https://github.com/grpc/grpc-go/blob/v1.58.0/examples/helloworld/helloworld/helloworld.proto)
gRPC service.

The code in this repository in not recommended for production deployments, and
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
variable, the intermediary service appends its own host name and zone, before
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
[creating a GKE cluster and Artifact Registry repository](docs/gke.md), and
creating a multi-node Kubernetes cluster using [kind](docs/kind.md).

If you want to build and deploy the Java control plane and sample application,
you need [Java 17](https://adoptium.net/).

If you want to build and deploy the Go control plane and sample application,
you need [Go 1.20](https://go.dev/doc/install) or later.

You also need the following tools:

- [kubectl](https://kubernetes.io/docs/reference/kubectl/)
- [Kustomize](https://kustomize.io/)
- [Skaffold](https://skaffold.dev/)
- A gRPC command-line client, such as
  [`grpcurl`](https://github.com/fullstorydev/grpcurl).

If you have already installed the
[Google Cloud SDK](https://cloud.google.com/sdk/docs/install),
you can install Skaffold, kubectl, and Kustomize using the `gcloud` command:

```shell
gcloud components install kubectl kustomize skaffold
```

Follow the steps in the documents
[Verify local development setup using Go](docs/verify-local-setup-go.md) or
[Verify local development setup using Java](docs/verify-local-setup-java.md)
to ensure that your cluster and the prerequisite tools are set up correctly.

To be continued...

## Kubernetes references

- [Kubernetes API Concepts](https://kubernetes.io/docs/reference/using-api/api-concepts/)
- [Headless Services](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services)
- [EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)

## xDS references

- [xDS API Overview](https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/overview)
- [xDS REST and gRPC protocol](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [Aggregated Discovery Service (ADS)](https://www.envoyproxy.io/docs/envoy/latest/configuration/overview/xds_api#aggregated-discovery-service)
- [gRFC A27: xDS-Based Global Load Balancing](https://github.com/grpc/proposal/blob/972b69ab1f0f7f6079af81a8c2b8a01a15ce3bec/A27-xds-global-load-balancing.md)
- [gRFC A36: xDS-Enabled Servers](https://github.com/grpc/proposal/blob/fd10c1a86562b712c2c5fa23178992654c47a072/A36-xds-for-servers.md)
- [xDS Features in gRPC](https://github.com/grpc/grpc/blob/1b31c6e0ba711787c05e8e78719896a682fca102/doc/grpc_xds_features.md)

## Disclaimer

This is not an officially supported Google product.
