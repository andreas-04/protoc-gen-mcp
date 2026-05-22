# Reference

Deep reference for `protoc-gen-mcp`. The [README](../README.md) covers
"what is it" and "how do I try it"; this page covers "how do I tune it".

## Two deployment patterns

| Pattern | When | Entry points |
|---------|------|--------------|
| **Embed in gRPC** (recommended) | One process serves gRPC and MCP. Same code owns the service impl. | `NewServer`, `RegisterLocal`, `ServeHTTP` |
| **Standalone binary** | MCP runs as its own deployment, dialing a remote gRPC backend. | `Main`, `Run`, `Register` |

Both speak the MCP streamable-HTTP transport. Stdio is intentionally not
supported — Claude Code, Claude Desktop, and the current MCP spec all
support HTTP, and dropping stdio keeps the API one path wide.

## Embedded pattern

```go
mcpSrv := mcpserver.NewServer()
mcpserver.RegisterLocal(mcpSrv, mcpserver.Impls{
    GreeterService: greeterImpl,
    BillingService: billingImpl,
})
go mcpserver.ServeHTTP(ctx, mcpSrv, ":8080")
```

`Impls` has one field per service across every proto file in the buf run.
Nil fields are skipped, so you can wire a subset.

### How it works

`RegisterLocal` constructs a `New<Service>LocalClient(impl)` adapter for
each populated field and hands it to the existing
`Register<Service>Tools(s, client)` helper. The adapter implements the
gRPC client interface by dispatching every call straight to the server
impl — no marshal/unmarshal, no `*grpc.ClientConn`, no port.

If you only have one service or want hand-rolled wiring:

```go
pb.RegisterGreeterServiceTools(mcpSrv, pb.NewLocalGreeterServiceClient(greeterImpl))
```

## Standalone pattern

### `mcpserver.Main()` — full drop-in

The whole binary. Parses flags, installs SIGINT/SIGTERM handling, dials
gRPC, serves MCP over HTTP, exits with the appropriate status.

### `mcpserver.Run(ctx, mcpserver.Config)` — embedded use

Same behavior as `Main`, but you control flags, config sources, and the
lifecycle.

```go
cfg := mcpserver.DefaultConfig()
cfg.GRPCAddr = os.Getenv("BACKEND_ADDR")
cfg.HTTPAddr = ":9000"
cfg.GRPCMetadata = []string{"x-tenant=" + tenant}

if err := mcpserver.Run(ctx, cfg); err != nil {
    log.Fatal(err)
}
```

### `mcpserver.Register(s, conn)` — bring-your-own server

When you already own the `*mcp.Server` lifecycle but want the
client-dialed flavor (e.g. embedding into a larger MCP server with
hand-written tools).

## Runtime flags

`mcpserver.Main()` accepts these flags (also visible via `mcp-server -h`):

| Flag | Default | Purpose |
|------|---------|---------|
| `-grpc-addr` | `localhost:50051` | gRPC backend address. |
| `-http-addr` | `:8080` | MCP HTTP listen address. |
| `-grpc-tls` | `false` | Dial the backend over TLS. |
| `-grpc-tls-insecure-skip-verify` | `false` | Skip server cert verification (dev only). |
| `-grpc-tls-ca-file` | `""` | PEM file with additional trusted CAs. |
| `-grpc-tls-server-name` | `""` | Override TLS SNI / verification hostname. |
| `-grpc-metadata` | `""` | Repeatable `key=value` forwarded as outgoing gRPC metadata. |
| `-version` | — | Print server name + version and exit. |

## Plugin options

Options go under `opt:` in `buf.gen.yaml`.

| Option | Default | Purpose |
|--------|---------|---------|
| `gen_aggregator` | `true` | Set to `false` to skip emitting `mcpserver/mcpserver.go` and only get the per-proto library files. |
| `aggregator_dir` | `mcpserver` | Directory (relative to `out:`) where the aggregator is written. |
| `aggregator_pkg` | derived | Full Go import path of the aggregator. Auto-derived as the longest common prefix of generated proto packages plus `aggregator_dir`. Set explicitly when auto-derivation can't find a common parent. |
| `server_name` | `<first-proto-basename>-mcp` | MCP `Implementation.Name` reported on Initialize. |
| `server_version` | `1.0.0` | MCP `Implementation.Version` reported on Initialize. |
| `grpc_package` | — | Override the Go import path for the gRPC-generated package referenced from the aggregator. Rarely needed; only set if `go_package` doesn't match the path you want imported. |

## Multi-proto behavior

Run `protoc-gen-mcp` over `auth.proto`, `users.proto`, and `billing.proto`
in the same `buf generate` invocation and the aggregator imports all
three and exposes a single `Impls` struct + `RegisterLocal` (or
`Register` for the standalone path) that wires every service. The
aggregator's Go import path is derived from the longest common prefix of
the three packages — e.g. `github.com/acme/api/gen/{auth,users,billing}`
→ aggregator at `github.com/acme/api/gen/mcpserver`.

If your protos sit under separate roots (no common parent), the plugin
exits with `could not derive aggregator package path` and you should set
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

Scalar fields are emitted **without** `omitempty` so an explicit
`false`, `0`, or `""` from the MCP client survives all the way to
`protojson.Unmarshal`. Pointer, slice, and `json.RawMessage` fields get
`omitempty` because their zero value (nil) is meaningfully distinct from
"field present and empty".
