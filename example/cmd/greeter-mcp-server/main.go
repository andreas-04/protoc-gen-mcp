// Command greeter-mcp-server runs the gRPC service and the MCP HTTP server
// in a single process — the recommended deployment pattern. Edit your own
// gRPC main like this to expose your service to LLM clients.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os/signal"
	"syscall"

	pb "github.com/andreas-04/buf-gen-mcp/example/gen/greeter"
	"github.com/andreas-04/buf-gen-mcp/example/gen/mcpserver"
	"github.com/andreas-04/buf-gen-mcp/example/internal/greeterimpl"
	"google.golang.org/grpc"
)

func main() {
	grpcAddr := flag.String("grpc-addr", ":50051", "gRPC listen address")
	mcpAddr := flag.String("mcp-addr", ":8080", "MCP HTTP listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// gRPC service — your existing setup, unchanged.
	greeter := greeterimpl.Server{}
	grpcSrv := grpc.NewServer()
	pb.RegisterGreeterServiceServer(grpcSrv, greeter)

	grpcLn, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		log.Fatalf("gRPC listen: %v", err)
	}
	go func() {
		log.Printf("greeter gRPC server listening on %s", grpcLn.Addr())
		if err := grpcSrv.Serve(grpcLn); err != nil {
			log.Fatalf("gRPC serve: %v", err)
		}
	}()
	defer grpcSrv.GracefulStop()

	// MCP server — three new lines.
	mcpSrv := mcpserver.NewServer()
	mcpserver.RegisterLocal(mcpSrv, mcpserver.Impls{GreeterService: greeter})
	if err := mcpserver.ServeHTTP(ctx, mcpSrv, *mcpAddr); err != nil {
		log.Fatalf("MCP serve: %v", err)
	}
}
