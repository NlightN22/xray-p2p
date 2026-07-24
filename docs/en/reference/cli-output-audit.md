# CLI output audit

The executable inventory is checked against the real Cobra command tree by:

```sh
make command-map
```

Every executable leaf must be explicitly present in both the classification
inventory (`go/cmd/xp2p/root/output_inventory.go`) and the per-command audit
inventory (`go/cmd/xp2p/root/output_audit*.go`). There is no default classification
or audit template. The meta-test fails for a new or stale entry in either inventory.
Generated maps in `commands_map/` expose the resulting classification and exception
reason.

## Audit matrix

| Command family | Operation | Human stdout | stderr sources | Runtime | Credential result | Interactive / streaming | Known repository consumers |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `client/server * list`, `state`, `status`, `obs`, `heartbeat contract` | read-only | tables or records | validation, API and filesystem diagnostics | command-specific; heartbeat contract needs none | no; explicit `--link` list output is intentional | state watch is rejected in JSON mode | host state, deploy, heartbeat, and migration tests |
| `client/server install`, `client deploy` | mutation | progress and summary | validation, install and service diagnostics | host/service integration | server install and deploy intentionally return generated credentials | prompts are disabled in JSON mode | Windows, Linux, and OpenWrt install/tunnel tests |
| `server user add/rotate`, identity provision | mutation | credential lines/link | validation and apply diagnostics | Desired/Live configuration | intentional public result | ambiguous selection must use flags in JSON mode | credential migration and tunnel fixtures |
| add/remove/enable/disable/mode/service actions | mutation/lifecycle action | confirmation or silence | validation, apply and service diagnostics | configuration and sometimes running service | no | destructive prompts are disabled in JSON mode | host service and routing tests |
| `completion`, `docs command-map`, `* render xray` | generator | shell, Markdown, or standalone JSON | generator diagnostics | none or configuration | no | bounded standalone document; `--json` rejected | build/release tooling |
| `diag`, `ping`, `client/server run`, service run, `server deploy` | lifecycle/streaming | event or foreground process stream | runtime logs and diagnostics | running process/runtime | no | streaming or foreground; `--json` rejected | services and diagnostic tests |

The explicit Go audit inventory stores a literal record for every leaf command. Each
record names the operation, concrete Go stdout and stderr sources, runtime
requirements, credential policy, interaction model, and known consumer scope. The
family table above is only a summary. A meta-test rejects missing, stale, duplicate,
or incomplete records and cross-checks them against the independent classification
inventory. Platform-specific leaves are registered individually, even when their
implementation is unavailable on the current host.

## Classification

| Class | Operation | JSON behavior |
| --- | --- | --- |
| `json` | read-only, mutation, credential result, or bounded lifecycle action | one success document on stdout |
| `generator` | shell, Markdown, Xray JSON, or file generator | explicit structured rejection |
| `lifecycle` | foreground service or listener | explicit structured rejection |
| `streaming` | multiple or continuous events | explicit rejection until NDJSON is defined |

## Output sources

The audit covers Cobra and standard-library flag parsing, direct writes to stdout,
Cobra writers, tabular renderers, generated JSON/TOML/link output, credentials,
warnings, diagnostics, logs, prompts, and commands that are silent on success.

The JSON presentation wrapper discards legacy rendering and accepts only a typed
result. Result-bearing handlers publish their domain model directly. Each
payload-free mutation has an explicit inventory adapter that publishes the common
`status`, `operation`, and affected `entity` model after its handler succeeds; the
wrapper never invents a result. JSON execution forces `--quiet` for commands whose human mode can prompt,
while commands without a quiet path require explicit selectors. Both legacy stdout
and handler stderr are isolated while the handler runs. On failure, the wrapper
restores the streams and writes exactly one structured error document to stderr.
Generator and lifecycle exclusions are rejected before their handlers run.

## Internal consumers

Repository searches for CLI output parsing must include host tests, shell scripts,
documentation, release checks, and UI bindings. UI code continues to call internal
use cases and must not shell out to `xp2p`. Human-output compatibility is retained
even when an internal test or automation consumer migrates to JSON.

## Credential policy

Commands whose purpose is to create, rotate, provision, export, or explicitly show a
credential retain that credential in their result. Status, list, health, diagnostic,
warning, log, and error output must not acquire credentials as a side effect of JSON
rendering.
