// Command protoc-gen-mcp is a protoc plugin that generates MCP (Model Context
// Protocol) server bindings for the gRPC services defined in a proto file.
//
// See the project README for a list of supported opt: parameters.
package main

import (
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/andreas-04/buf-gen-mcp/internal/generator"
)

// version is the protoc-gen-mcp release. Kept in sync with buf.plugin.yaml
// and reported by `protoc-gen-mcp -version`.
const version = "v0.1.0"

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
