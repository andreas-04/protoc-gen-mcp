# buf-gen-mcp

`protoc-gen-mcp` turns your existing gRPC services into [MCP][mcp] tools.
Each unary RPC becomes an MCP tool with a JSON-schema-typed input that maps
back onto the proto message via `protojson`, so the LLM-facing surface stays
in lock-step with your `.proto` files.

Streaming RPCs are intentionally skipped — MCP's tool model is request/response.

[mcp]: https://modelcontextprotocol.io

## What it generates

For every proto file with at least one unary RPC, you get:

| File | Purpose |
|------|---------|
| `<proto>_mcp.pb.go` (next to your `*.pb.go`) | Per-service `Register<Service>Tools(*mcp.Server, <Service>Client)` plus typed `*Input` structs. |
| `mcpserver/mcpserver.go` (once per buf run) | Drop-in aggregator: `Main()`, `Run(ctx, Config)`, and `Register(srv, conn)` wiring every service across every proto file. |

The aggregator is what lets your `main.go` shrink to a single line:

```go
package main

import "your.module/gen/mcpserver"

func main() { mcpserver.Main() }
```

That binary speaks MCP over stdio by default and accepts standard flags for
HTTP transport, TLS-to-backend, and outgoing gRPC metadata. See
[Drop-in server](#drop-in-server) below.

## Quickstart

The repo's `example/` directory is a working greeter — clone it, follow the
five steps, and you'll have a runnable MCP server.

### 1. Add the plugin to `buf.gen.yaml`

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: [paths=source_relative]

  - remote: buf.build/grpc/go
    out: gen
    opt: [paths=source_relative]

  # Build first: `go install github.com/andreas-04/buf-gen-mcp/cmd/protoc-gen-mcp@latest`
  - local: protoc-gen-mcp
    out: gen
    opt: [paths=source_relative]
```

### 2. Generate

```sh
buf generate
```

This drops `gen/<pkg>/<pkg>_mcp.pb.go` next to the gRPC stubs and
`gen/mcpserver/mcpserver.go` once.

### 3. Wire your `main.go`

```go
// cmd/mcp-server/main.go
package main

import "your.module/gen/mcpserver"

func main() { mcpserver.Main() }
```

### 4. Run it

```sh
go run ./cmd/mcp-server -grpc-addr api.internal:50051
```

### 5. Point an MCP client at it

For Claude Code / Claude Desktop:

```json
{
  "mcpServers": {
    "my-service": {
      "type": "stdio",
      "command": "/abs/path/to/mcp-server",
      "args": ["-grpc-addr", "api.internal:50051"]
    }
  }
}
```

## Installation

```sh
go install github.com/andreas-04/buf-gen-mcp/cmd/protoc-gen-mcp@latest
```

`protoc-gen-mcp` is a standalone protoc plugin — `buf` and `protoc` both
support it.

## Drop-in server

The generated `mcpserver` package exposes three entry points; pick the level
of control you need.

### `mcpserver.Main()` — full drop-in

The whole binary. Parses flags, installs SIGINT/SIGTERM handling, dials gRPC,
runs the chosen transport, exits with the appropriate status.

| Flag | Default | Purpose |
|------|---------|---------|
| `-grpc-addr` | `localhost:50051` | gRPC backend address. |
| `-transport` | `stdio` | `stdio` or `http`. |
| `-http-port` | `8080` | Listen port when `-transport=http`. |
| `-grpc-tls` | `false` | Dial the backend over TLS. |
| `-grpc-tls-insecure-skip-verify` | `false` | Skip server cert verification (dev only). |
| `-grpc-tls-ca-file` | `""` | PEM file with additional trusted CAs. |
| `-grpc-tls-server-name` | `""` | Override TLS SNI / verification hostname. |
| `-grpc-metadata` | `""` | Repeatable `key=value` forwarded as outgoing gRPC metadata. |
| `-version` | — | Print server name + version and exit. |

### `mcpserver.Run(ctx, mcpserver.Config)` — embedded use

Same behavior as `Main`, but you control flags, config sources, and the
lifecycle. Cancel `ctx` to shut down.

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()

cfg := mcpserver.DefaultConfig()
cfg.GRPCAddr = os.Getenv("BACKEND_ADDR")
cfg.Transport = "http"
cfg.HTTPPort = 9000
cfg.GRPCMetadata = []string{"x-tenant=" + tenant}

if err := mcpserver.Run(ctx, cfg); err != nil {
    log.Fatal(err)
}
```

### `mcpserver.Register(*mcp.Server, grpc.ClientConnInterface)` — bring your own server

When you already own the `*mcp.Server` lifecycle (custom transport, embedded
in a larger MCP server with hand-written tools, etc.) just call `Register`:

```go
srv := mcp.NewServer(&mcp.Implementation{Name: "my-mcp", Version: "1.0.0"}, nil)
mcpserver.Register(srv, conn)

// add your own tools, prompts, resources here…

_ = srv.Run(ctx, &mcp.StdioTransport{})
```

## Plugin options

All options are key=value pairs in `buf.gen.yaml` under `opt:`.

| Option | Default | Purpose |
|--------|---------|---------|
| `gen_aggregator` | `true` | Set to `false` to skip emitting `mcpserver/mcpserver.go` and only get the per-proto library files. |
| `aggregator_dir` | `mcpserver` | Directory (relative to `out:`) where the aggregator is written. |
| `aggregator_pkg` | derived | Full Go import path of the aggregator. Auto-derived as the longest common prefix of generated proto packages plus `aggregator_dir`. Set explicitly when auto-derivation can't find a common parent. |
| `server_name` | `<first-proto-basename>-mcp` | MCP `Implementation.Name` reported on Initialize. |
| `server_version` | `0.1.0` | MCP `Implementation.Version` reported on Initialize. |
| `grpc_package` | — | Override the Go import path for the gRPC-generated package referenced from the aggregator. Rarely needed; only set if `go_package` doesn't match the path you want imported. |

## Multi-proto behavior

Run protoc-gen-mcp over `auth.proto`, `users.proto`, and `billing.proto` in
the same `buf generate` invocation and the aggregator imports all three and
wires every service in one `Register` call. The aggregator's Go import path
is derived from the longest common prefix of the three packages — e.g.
`github.com/acme/api/gen/{auth,users,billing}` → aggregator at
`github.com/acme/api/gen/mcpserver`.

If your protos sit under separate roots (no common parent), the plugin will
exit with `could not derive aggregator package path` and you should set
`aggregator_pkg` explicitly.

## Type mapping

| Proto type | Go type | JSON shape |
|------------|---------|------------|
| `bool`, `int32`, `int64`, `uint32`, `uint64`, `float`, `double`, `string` | matching Go scalar | matching JSON scalar |
| `enum` | `string` | enum name (LLM-friendly) |
| `bytes` | `string` | base64 |
| `repeated T` | `[]T` | JSON array |
| `map<K, V>` | `json.RawMessage` | JSON object |
| nested message | `json.RawMessage` | JSON object (round-tripped through protojson) |
| `google.protobuf.Timestamp` | `string` | RFC 3339 |
| `google.protobuf.Duration` | `string` | e.g. `"10s"` |
| `google.protobuf.*Value` wrappers | `*T` | scalar or absent |
| `google.protobuf.Struct` / `Value` / `ListValue` / `Any` | `json.RawMessage` | opaque JSON |
| `google.protobuf.Empty` | `struct{}` | `{}` |

Scalar fields are emitted **without** `omitempty` so an explicit `false`, `0`,
or `""` from the MCP client survives all the way to `protojson.Unmarshal`.
Pointer, slice, and `json.RawMessage` fields get `omitempty` because their
zero value (nil) is meaningfully distinct from "field present and empty".

## Limitations

- **Streaming RPCs are skipped.** Server-streaming, client-streaming, and
  bidi methods have no MCP analogue and are filtered out.
- **`oneof` fields** are represented as `json.RawMessage` per branch. You can
  still call them via raw JSON; full schema generation for `oneof` is a
  candidate for a future release.
- **Nested messages** also become `json.RawMessage`. The JSON Schema you see
  on the MCP side is therefore "object" without further detail. The wire
  round-trip through protojson still validates the payload.

## Project status

`v0.1.0`. The wire format and option names are stable across patch releases
inside `0.x`; expect renames at `1.0` based on early feedback.

## Development

```sh
make build           # build ./protoc-gen-mcp
make test            # run unit tests
make test-example    # run end-to-end integration tests against the example
make generate-example # rebuild plugin and regenerate the greeter example
make build-example   # build example/bin/{greeter-server,mcp-server}
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the loop in more detail.

## License

Apache-2.0 — see [LICENSE](LICENSE).
