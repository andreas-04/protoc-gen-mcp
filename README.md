# protoc-gen-mcp

[![Go Reference](https://pkg.go.dev/badge/github.com/andreas-04/protoc-gen-mcp@v1.0.2.svg)](https://pkg.go.dev/github.com/andreas-04/protoc-gen-mcp@v1.0.2)
[![CI](https://github.com/andreas-04/protoc-gen-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/andreas-04/protoc-gen-mcp/actions/workflows/ci.yml)

A protoc plugin that turns your existing gRPC services into [MCP][mcp] tools so
LLM clients can call them directly. Every unary RPC becomes a typed MCP
tool; you don't write or maintain any glue code.

[mcp]: https://modelcontextprotocol.io

## What you get

Run `buf generate` and you'll have:

- a per-proto `*_mcp.pb.go` library with typed `Register<Service>Tools` helpers and an in-process adapter, and
- a `mcpserver` package with `NewServer`, `RegisterLocal`, and `ServeHTTP` that drop into your existing gRPC main.

 The
generated MCP server speaks streamable HTTP the transport modern MCP clients use.

## Quickstart 

### 1. Install the plugin

```sh
go install github.com/andreas-04/protoc-gen-mcp/cmd/protoc-gen-mcp@latest
```

### 2. Add it to `buf.gen.yaml`

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: [paths=source_relative]
  - remote: buf.build/grpc/go
    out: gen
    opt: [paths=source_relative]
  - local: protoc-gen-mcp
    out: gen
    opt: [paths=source_relative]
```

### 3. Generate

```sh
buf generate
```

### 4. Edit your existing `main.go`

```go
import (
    "context"
    pb "your.module/gen/greeter"
    "your.module/gen/mcpserver"
)

func main() {
    ctx := context.Background()

    // Your existing gRPC setup, unchanged.
    greeter := &greeterServer{}
    grpcSrv := grpc.NewServer()
    pb.RegisterGreeterServiceServer(grpcSrv, greeter)
    go grpcSrv.Serve(lis)

    // Three new lines for MCP.
    mcpSrv := mcpserver.NewServer()
    mcpserver.RegisterLocal(mcpSrv, mcpserver.Impls{GreeterService: greeter})
    mcpserver.ServeHTTP(ctx, mcpSrv, ":8080")
}
```

### 5. Point an MCP client at it

```json
{
  "mcpServers": {
    "my-service": {
      "type": "http",
      "url": "http://localhost:8080"
    }
  }
}
```

Works with any  client speaking the MCP streamable-HTTP transport.

## When you want a separate process

For deployments where MCP lives apart from gRPC, `mcpserver.Main()` is a
standalone binary that dials a remote gRPC backend and serves MCP over
HTTP. Your `main.go` becomes:

```go
package main

import "your.module/gen/mcpserver"

func main() { mcpserver.Main() }
```

Run it with `-grpc-addr api.internal:50051 -http-addr :8080`. See
[`docs/reference.md`](docs/reference.md) for the full flag set.

## More detail

- Runtime flags — `mcp-server -h`
- Generated package API — `go doc your.module/gen/mcpserver`
- Plugin opts, type mapping, multi-proto behavior, TLS/auth — [`docs/reference.md`](docs/reference.md)

## Limitations

- **Streaming RPCs are skipped.** MCP tools are request/response.
- **`oneof` and nested message fields** are passed through as `json.RawMessage`.
  The wire round-trip through `protojson` still validates the payload.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the dev loop and CI.

## License

Apache-2.0 — see [LICENSE](LICENSE).
