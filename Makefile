.PHONY: build install test test-example generate-example build-example run-example clean

BINARY := protoc-gen-mcp

## build: compile the plugin binary into the repo root.
build:
	go build -o $(BINARY) ./cmd/protoc-gen-mcp

## install: install protoc-gen-mcp into $GOPATH/bin (or $GOBIN).
install:
	go install ./cmd/protoc-gen-mcp

## test: run all unit tests.
test:
	go test ./...

## test-example: spin up gRPC + MCP servers in-process and run end-to-end tests.
test-example:
	cd example && go test ./... -v -count=1 -timeout 60s

## generate-example: build the plugin locally then regenerate the example proto.
generate-example: build
	cd example && buf generate

## build-example: compile the example binaries used by .mcp.json and the docs.
build-example:
	cd example && go build -o bin/greeter-server     ./cmd/greeter-server
	cd example && go build -o bin/mcp-server         ./cmd/mcp-server
	cd example && go build -o bin/greeter-mcp-server ./cmd/greeter-mcp-server

## run-example: build and run the embedded greeter+MCP server (matches .mcp.json).
run-example: build-example
	./example/bin/greeter-mcp-server

## clean: remove the local binary.
clean:
	rm -f $(BINARY)
