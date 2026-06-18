# Xray API Proto Snapshot

This directory contains Xray-core API proto files and generated Go bindings used by `go/internal/xrayapi`.

- Upstream repository: https://github.com/XTLS/Xray-core
- Source version: 26.2.6
- Go module tag used for generated bindings: `github.com/xtls/xray-core@v1.260206.0`
- Updated: 2026-06-18

Included proto files:

- `app/stats/command/command.proto`

Generation command used upstream:

```sh
protoc --go_out=. --go-grpc_out=. app/stats/command/command.proto
```

Update this snapshot only together with an intentional Xray pin bump in `go/internal/xray/pinned.json`.
