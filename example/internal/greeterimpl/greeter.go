// Package greeterimpl is the shared GreeterService implementation used by
// the example binaries and integration tests. Real services would put this
// in their own internal/ tree; the example collects it here so the binaries
// stay focused on wiring.
package greeterimpl

import (
	"context"
	"fmt"

	pb "github.com/andreas-04/buf-gen-mcp/example/gen/greeter"
)

// Server is a tiny GreeterService that translates greetings by BCP-47
// language tag. Unknown languages fall back to English.
type Server struct {
	pb.UnimplementedGreeterServiceServer
}

func (Server) SayHello(_ context.Context, req *pb.SayHelloRequest) (*pb.SayHelloResponse, error) {
	greeting := "Hello"
	switch req.GetLanguage() {
	case "es":
		greeting = "Hola"
	case "fr":
		greeting = "Bonjour"
	case "de":
		greeting = "Hallo"
	}
	return &pb.SayHelloResponse{Message: fmt.Sprintf("%s, %s!", greeting, req.GetName())}, nil
}

func (Server) SayGoodbye(_ context.Context, req *pb.SayGoodbyeRequest) (*pb.SayGoodbyeResponse, error) {
	return &pb.SayGoodbyeResponse{Message: fmt.Sprintf("Goodbye, %s!", req.GetName())}, nil
}
