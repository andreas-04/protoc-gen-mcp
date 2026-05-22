# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] — 2026-05-21

Initial public release.

### Added
- `protoc-gen-mcp` plugin: generates MCP tool bindings for every unary RPC in
  every proto file with at least one service.
- Per-proto `Register<Service>Tools(*mcp.Server, <Service>Client)` helper and
  typed `*Input` structs that round-trip through `protojson`.
- Drop-in aggregator package (`mcpserver/mcpserver.go`) with three entry
  points: `Main()` (full binary), `Run(ctx, Config)` (embedded use), and
  `Register(srv, conn)` (bring-your-own-`*mcp.Server`).
- Stdio and streamable HTTP transports.
- TLS-to-backend flags: `-grpc-tls`, `-grpc-tls-insecure-skip-verify`,
  `-grpc-tls-ca-file`, `-grpc-tls-server-name`.
- Outgoing gRPC metadata via repeatable `-grpc-metadata key=value`.
- Plugin options: `gen_aggregator`, `aggregator_dir`, `aggregator_pkg`,
  `server_name`, `server_version`, `grpc_package`.
- Multi-proto aggregation: a single `mcpserver` package imports and wires
  every generated service across every proto file in one buf invocation.
- Well-known type mapping: Timestamp/Duration → string, wrapper messages →
  `*T`, Struct/Value/ListValue/Any → `json.RawMessage`, Empty → `struct{}`.
- Apache-2.0 license, README, CHANGELOG, CONTRIBUTING.

[Unreleased]: https://github.com/andreas-04/buf-gen-mcp/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/andreas-04/buf-gen-mcp/releases/tag/v0.1.0
