#!/usr/bin/env bash
# run-mcp.sh — launched by Claude Code as a stdio MCP server.
#
# Starts the greeter gRPC server in the background on an ephemeral port, then
# execs the MCP server in stdio mode pointing at that port.
# When Claude Code terminates this process the trap cleans up the gRPC server.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Pick a free TCP port by briefly binding to :0 and reading the assigned port.
GRPC_PORT=$(python3 -c '
import socket
s = socket.socket()
s.bind(("localhost", 0))
print(s.getsockname()[1])
s.close()
')

# Start gRPC backend in background on that port.
"$SCRIPT_DIR/greeter-server" -addr ":${GRPC_PORT}" &
GRPC_PID=$!

# Kill the gRPC server when this script exits for any reason.
trap 'kill "$GRPC_PID" 2>/dev/null || true' EXIT

# grpc.NewClient is lazy — the connection is established on the first call, so
# there is no need to wait for the gRPC server to finish binding.
exec "$SCRIPT_DIR/mcp-server" -grpc-addr "localhost:${GRPC_PORT}"
