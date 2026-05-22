# Contributing

Thanks for the interest. This project is small and the loop is short.

## Prerequisites

- Go 1.23+ (the plugin module). The `example/` module uses Go 1.25.
- [`buf`](https://buf.build) on your `$PATH` (the example uses it to regenerate).

## Dev loop

```sh
make build           # compile ./protoc-gen-mcp into the repo root
make test            # unit tests for the plugin
make test-example    # end-to-end tests against the greeter example
make generate-example # rebuild plugin AND regenerate the example
make build-example   # build example/bin/{greeter-server,mcp-server,greeter-mcp-server}
make run-example     # build and run greeter-mcp-server (matches .mcp.json)
make clean
```

When you change templates in `internal/tmpl/`, run `make generate-example` so
the regenerated sources land in `example/gen/` and the integration tests
exercise the new output.

### Testing through Claude Code

`.mcp.json` at the repo root wires the example greeter into Claude Code over
the MCP streamable-HTTP transport. Run `make run-example` to start
`greeter-mcp-server` on `localhost:8080`; with that running, opening this
directory in Claude Code exposes the generated tools and you can exercise
them directly. The config is repo dev-tooling — ignore it if you don't use
Claude Code.

## Layout

```
cmd/protoc-gen-mcp/      # plugin entrypoint
internal/generator/      # protogen → template data
internal/tmpl/           # text/template sources, embedded via //go:embed
example/                 # working greeter; doubles as the integration harness
example/proto/           # source .proto
example/gen/             # generated stubs (committed)
example/cmd/             # user-written mains (greeter-server, mcp-server)
```

The `example/` module is intentionally a separate Go module so the plugin
itself stays dependency-free at runtime.

## Tests

There are three layers of tests; all of them run in CI.

- **Pure-function unit tests** under `internal/generator/*_test.go` run `go test ./internal/generator/...`
- **Golden-file template tests** in `internal/generator/templates_test.go`
  render `register.go.tmpl` and `aggregator.go.tmpl` against 
  fixtures and compare against `testdata/golden/*.golden`. Update with
  `go test ./internal/generator/... -update` (or `UPDATE_GOLDENS=1 go test
  ./internal/generator/...`). Commit the regenerated goldens with your PR.
- **End-to-end integration tests** in `example/integration_test.go` exercise
  the generated code over gRPC + MCP across stdio, in-memory, and HTTP
  transports, plus the aggregator's `Register` and `Run`. `make test-example`.

## CI

`.github/workflows/ci.yml` runs on every push to `main` and every PR:

- **test job** — `go vet` + `go test -race` on the plugin and the example.
- **drift job** — runs `make build && buf generate` in `example/`, then
  fails if `git status --porcelain example/gen` shows anything. If you
  changed templates or generator logic, regenerate locally with
  `make generate-example` and commit the result.

## Sending a change

1. Open an issue first for behavior changes or new opts — the surface is
   small and we'd rather discuss before you build.
2. Keep PRs scoped. Generator change + matching example regeneration +
   integration test in one PR is fine; mixing unrelated cleanups isn't.
3. Run `make test test-example` before pushing.
4. Generated files are committed — include the regen diff in your PR.

## Releasing (maintainer notes)

1. Bump the `version` constant in `cmd/protoc-gen-mcp/main.go`.
2. Tag `vX.Y.Z`, push the tag, and write the release notes in the GitHub
   release UI from the commit log since the previous tag.
3. The aggregator's default `ServerVersion` is set per-generation by the
   `server_version` opt; bumping the plugin doesn't bump downstream users.
