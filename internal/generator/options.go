package generator

import (
	"fmt"
	"strconv"
)

// Options holds plugin configuration passed via buf.gen.yaml opt: params.
//
// All options are key=value pairs supplied in buf.gen.yaml:
//
//	- local: ../protoc-gen-mcp
//	  out: gen
//	  opt:
//	    - paths=source_relative
//	    - aggregator_pkg=github.com/acme/api/gen/mcpserver
//	    - server_name=acme-mcp
//	    - server_version=1.2.3
type Options struct {
	// GRPCPackage overrides the Go import path used for the gRPC-generated
	// package in the aggregator. When empty the import path is taken from
	// each proto file's go_package option.
	GRPCPackage string

	// GenAggregator controls whether the drop-in 'mcpserver' aggregator file
	// is emitted. Default true. Set to false if you only want the per-proto
	// _mcp.pb.go libraries and intend to wire registration up by hand.
	GenAggregator bool

	// AggregatorDir is the directory (relative to the buf output dir) where
	// the aggregator file is written. Default 'mcpserver'.
	AggregatorDir string

	// AggregatorPkg is the full Go import path of the aggregator package.
	// When empty it is derived from the longest common prefix of the
	// generated proto packages' Go import paths, plus AggregatorDir.
	// Set this explicitly if auto-derivation does not match your module
	// layout (e.g. when proto files live outside a shared parent).
	AggregatorPkg string

	// ServerName is the MCP implementation name reported on Initialize.
	// Default: '<first proto basename>-mcp'.
	ServerName string

	// ServerVersion is the MCP implementation version reported on
	// Initialize. Default: '0.1.0'.
	ServerVersion string
}

// Default returns Options pre-populated with documented defaults. Use this
// when constructing Options programmatically; the plugin entry point applies
// the same defaults after parsing opt: parameters.
func Default() Options {
	return Options{
		GenAggregator: true,
		AggregatorDir: "mcpserver",
		ServerVersion: "0.1.0",
	}
}

// Set parses a single key=value parameter pair.
// The signature matches protogen.Options.ParamFunc.
func (o *Options) Set(name, value string) error {
	switch name {
	case "grpc_package":
		o.GRPCPackage = value
	case "gen_aggregator":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("protoc-gen-mcp: gen_aggregator=%q: %w", value, err)
		}
		o.GenAggregator = b
	case "aggregator_dir":
		o.AggregatorDir = value
	case "aggregator_pkg":
		o.AggregatorPkg = value
	case "server_name":
		o.ServerName = value
	case "server_version":
		o.ServerVersion = value
	default:
		return fmt.Errorf("protoc-gen-mcp: unknown parameter %q", name)
	}
	return nil
}
