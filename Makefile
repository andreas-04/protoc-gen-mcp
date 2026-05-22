.PHONY: build install test test-example generate-example clean

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

## clean: remove the local binary.
clean:
	rm -f $(BINARY)
