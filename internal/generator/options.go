package generator

import "fmt"

// Options holds plugin configuration passed via buf.gen.yaml opt: params.
type Options struct {
	// GRPCPackage overrides the Go import path used for the gRPC-generated
	// package in the standalone server. When empty the import path is taken
	// directly from the proto file's go_package option.
	GRPCPackage string
}

// Set parses a single key=value parameter pair.
// The signature matches protogen.Options.ParamFunc.
func (o *Options) Set(name, value string) error {
	switch name {
	case "grpc_package":
		o.GRPCPackage = value
	default:
		return fmt.Errorf("protoc-gen-mcp: unknown parameter %q", name)
	}
	return nil
}
