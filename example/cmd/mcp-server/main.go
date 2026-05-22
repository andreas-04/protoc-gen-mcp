// Command mcp-server is the example's one-line drop-in MCP server.
//
// Everything — flag parsing, signal handling, transport selection, TLS,
// outgoing metadata — is handled inside mcpserver.Main. If you need more
// control, swap Main() for a call to mcpserver.Run with a custom Config.
package main

import "github.com/andreas-04/protoc-gen-mcp/example/gen/mcpserver"

func main() { mcpserver.Main() }
