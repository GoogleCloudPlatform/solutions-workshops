# control-plane-java

Sample xDS control plane management server for gRPC applications running on
Kubernetes, implemented in Java.

The goal of this implementation is to provide a practical understanding of xDS,
beyond the API specifications and the
[pithy examples](https://github.com/grpc/grpc-java/tree/v1.59.0/examples/example-xds).

This sample implementation is not recommended for production deployments.
If you want a production-ready xDS control plane, we recommend
[Cloud Service Mesh](https://cloud.google.com/service-mesh/docs/service-routing/proxyless-overview)
from Google Cloud.

## References

- [gRPC xDS example](https://github.com/grpc/grpc-java/tree/v1.59.0/examples/example-xds)
- [Example xDS Server](https://github.com/envoyproxy/java-control-plane/blob/v1.0.39/server/src/test/java/io/envoyproxy/controlplane/server/TestMain.java)

## Disclaimer

This is not an officially supported Google product.
