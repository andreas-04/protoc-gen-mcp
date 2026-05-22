// Command greeter-server is a minimal gRPC-only server that implements
// GreeterService. Used by the standalone MCP setup (cmd/mcp-server dials it
// over the network) and by Makefile targets that need a backend.
package main

import (
	"flag"
	"log"
	"net"

	pb "github.com/andreas-04/protoc-gen-mcp/example/gen/greeter"
	"github.com/andreas-04/protoc-gen-mcp/example/internal/greeterimpl"
	"google.golang.org/grpc"
)

func main() {
	addr := flag.String("addr", ":50051", "gRPC listen address")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGreeterServiceServer(s, greeterimpl.Server{})
	log.Printf("greeter gRPC server listening on %s", *addr)
	if err := s.Serve(ln); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
