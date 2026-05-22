// Command protoc-gen-mcp is a protoc plugin that generates MCP (Model Context
// Protocol) server bindings for the gRPC services defined in a proto file.
//
// See the project README for a list of supported opt: parameters.
package main

import (
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/andreas-04/protoc-gen-mcp/internal/generator"
)

// version is the protoc-gen-mcp release and is reported by
// `protoc-gen-mcp -version`.
const version = "v1.0.0"

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println("protoc-gen-mcp", version)
		return
	}

	opts := generator.Default()
	protogen.Options{
		ParamFunc: opts.Set,
	}.Run(func(gen *protogen.Plugin) error {
		// Advertise proto3 optional support — without this protoc/buf warn
		// every time a .proto file uses the 'optional' keyword.
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		g := generator.New(opts)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if len(f.Services) == 0 {
				continue
			}
			if err := g.AddFile(gen, f); err != nil {
				return err
			}
		}
		return g.Finalize(gen)
	})
}
