// Package protocgenmcp documents the module root for protoc-gen-mcp.
//
// protoc-gen-mcp is a protoc plugin that turns unary gRPC services into MCP
// tools. Most users interact with the project by installing and running the
// command in cmd/protoc-gen-mcp and by importing the generated Go code emitted
// by the plugin.
//
// The module root intentionally exposes documentation only. The plugin
// implementation itself lives under internal/, and the generated output is the
// public surface consumed by downstream services.
package protocgenmcp
