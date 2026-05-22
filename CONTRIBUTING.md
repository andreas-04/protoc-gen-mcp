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
make build-example   # build example/bin/{greeter-server,mcp-server} (used by .mcp.json)
make clean
```

When you change templates in `internal/tmpl/`, run `make generate-example` so
the regenerated sources land in `example/gen/` and the integration tests
exercise the new output.

### Testing through Claude Code

`.mcp.json` at the repo root wires the example greeter into Claude Code as a
stdio MCP server. Run `make build-example` once to produce
`example/bin/greeter-server` and `example/bin/mcp-server`; after that, opening
this directory in Claude Code exposes the generated tools and you can exercise
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

## Sending a change

1. Open an issue first for behavior changes or new opts — the surface is
   small and we'd rather discuss before you build.
2. Keep PRs scoped. Generator change + matching example regeneration +
   integration test in one PR is fine; mixing unrelated cleanups isn't.
3. Run `make test test-example` before pushing.
4. Generated files are committed — include the regen diff in your PR.

## Releasing (maintainer notes)

1. Bump the `version` constant in `cmd/protoc-gen-mcp/main.go` and the
   `plugin_version` field in `buf.plugin.yaml`.
2. Move `Unreleased` entries to a new section in `CHANGELOG.md`.
3. Tag `vX.Y.Z`, push the tag.
4. The aggregator's default `ServerVersion` is set per-generation by the
   `server_version` opt; bumping the plugin doesn't bump downstream users.
