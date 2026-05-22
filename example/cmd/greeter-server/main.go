// Command greeter-server is a minimal gRPC server that implements GreeterService.
// It is used to test the generated MCP server end-to-end.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	pb "github.com/andreas-04/buf-gen-mcp/example/gen/greeter"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedGreeterServiceServer
}

func (s *server) SayHello(_ context.Context, req *pb.SayHelloRequest) (*pb.SayHelloResponse, error) {
	greeting := "Hello"
	switch req.GetLanguage() {
	case "es":
		greeting = "Hola"
	case "fr":
		greeting = "Bonjour"
	case "de":
		greeting = "Hallo"
	}
	return &pb.SayHelloResponse{
		Message: fmt.Sprintf("%s, %s!", greeting, req.GetName()),
	}, nil
}

func (s *server) SayGoodbye(_ context.Context, req *pb.SayGoodbyeRequest) (*pb.SayGoodbyeResponse, error) {
	return &pb.SayGoodbyeResponse{
		Message: fmt.Sprintf("Goodbye, %s!", req.GetName()),
	}, nil
}

func main() {
	addr := flag.String("addr", ":50051", "gRPC listen address")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGreeterServiceServer(s, &server{})
	log.Printf("greeter gRPC server listening on %s", *addr)
	if err := s.Serve(ln); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
