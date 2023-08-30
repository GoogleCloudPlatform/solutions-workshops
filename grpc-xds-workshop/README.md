# grpc-xds-workshop

This directory contains sample implementations of
[xDS](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/dynamic_configuration)
control plane management servers for gRPC applications running on Kubernetes
clusters.

The implementations aim to provide a practical understanding of xDS,
beyond the 
[API references](https://www.envoyproxy.io/docs/envoy/latest/api-v3/api)
and the
[pithy](https://github.com/grpc/grpc-go/tree/v1.57.0/examples/features/xds)
[examples](https://github.com/grpc/grpc-java/tree/v1.57.1/examples/example-xds).

## Prerequisites

To run the control plane server, you need a Kubernetes cluster. We recommend
[Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/docs),
but you can also use other clusters, including local clusters using
[kind](https://kind.sigs.k8s.io/).

The [`docs`](docs) directory contains instructions on creating a
[GKE cluster](docs/gke.md), and a multi-node Kubernetes cluster using
[kind](docs/kind.md).

You also need the following tools:

- [Java 17](https://adoptium.net/)
- [kubectl](https://kubernetes.io/docs/reference/kubectl/)
- [Kustomize](https://kustomize.io/)
- [Skaffold](https://skaffold.dev/)
- A gRPC command-line client, such as
  [`grpcurl`](https://github.com/fullstorydev/grpcurl).

If you want to run the Go implementation of the control plane server,
you also need [Go 1.20 or later](https://go.dev/).

If you have already installed the
[Google Cloud SDK](https://cloud.google.com/sdk/docs/install),
you can install Skaffold, kubectl, and Kustomize using the `gcloud` command:

```shell
gcloud components install kubectl kustomize skaffold
```

Follow the steps in the
[local development setup guide](docs/verify-local-setup.md) to ensure
that your cluster and the prerequisite tools are set up correctly.

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
