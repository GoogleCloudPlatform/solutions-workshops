# control-plane-go

Sample xDS control plane management server for gRPC applications running on
Kubernetes, implemented in Go.

The goal of this implementation is to provide a practical understanding of xDS,
beyond the API specifications and the
[pithy examples](https://github.com/grpc/grpc-go/tree/v1.59.0/examples/features/xds).

This sample implementation is not recommended for production deployments, and
there are no plans to make it production-ready. Instead, we recommend
[Traffic Director](https://cloud.google.com/traffic-director/docs) from Google Cloud.

## References

- [gRPC xDS example](https://github.com/grpc/grpc-go/tree/v1.59.0/examples/features/xds)
- [Example xDS Server](https://github.com/envoyproxy/go-control-plane/tree/v0.11.1/internal/example)
- [Example Envoy control plane](https://github.com/envoyproxy/go-control-plane/tree/v0.11.1/examples/dyplomat)

## Disclaimer

This is not an officially supported Google product.
