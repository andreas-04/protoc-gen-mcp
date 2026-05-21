package main
package main

import (
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/andreasneacsu/buf-gen-mcp/internal/generator"
)

const version = "v0.1.0"

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println("protoc-gen-mcp", version)
		return
	}

	var opts generator.Options
	protogen.Options{
		ParamFunc: opts.Set,
	}.Run(func(gen *protogen.Plugin) error {
		// Track whether the standalone server has been generated already.
		// With strategy: directory, multiple files may arrive in a single
		// invocation; we only emit cmd/mcp-server/main.go once.
		serverGenerated := false

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if len(f.Services) == 0 {
				continue
			}
			if err := generator.GenerateFile(gen, f, opts, !serverGenerated); err != nil {
				return err
			}
			serverGenerated = true
		}
		return nil
	})
}
