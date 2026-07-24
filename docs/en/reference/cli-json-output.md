# CLI JSON output

Human-readable output remains the default. Automation can request the versioned CLI
contract with the global `--json` (`-J`) flag:

```sh
xp2p --json client list
```

This flag controls command results. It is independent from `--log-json`, which only
controls log formatting.

## Success contract

stdout contains exactly one JSON document followed by a newline:

```json
{
  "schema_version": "1",
  "command": "xp2p client list",
  "result": {}
}
```

`schema_version` versions the public CLI output contract, not Desired, Live, or
persisted state. Numbers and booleans remain JSON numbers and booleans. Commands
publish command-specific typed objects or arrays as `result`. Human-readable output
is never captured into a `text` field. Payload-free mutations use
`{"status":"completed"}`. List results use an empty array rather than `null`.

Times are UTC RFC 3339 strings. Durations and byte counts are JSON numbers in the
unit named by the field (for example, `latency_ms` and `upload_bytes`). CIDRs, hosts,
and `host:port` targets remain strings. Optional scalar values are either omitted or
`null` as defined by the command result model.

Contract version 1 follows these compatibility rules:

- new optional fields may be added;
- existing fields are not removed or renamed;
- field types do not change;
- breaking changes require a new contract version and an upgrade note.

## Error contract

A failure preserves its non-zero exit code, leaves stdout empty, and writes exactly
one JSON document to stderr:

```json
{
  "schema_version": "1",
  "command": "xp2p client list",
  "error": {
    "code": "command_failed",
    "message": "configuration is unavailable"
  }
}
```

`invalid_argument`, `unsupported_output_format`, `missing_json_result`, and
`command_failed` are stable machine-readable codes. Handler logs, warnings, prompts,
progress messages, Cobra usage, and ANSI formatting are suppressed. A failed
command leaves stdout empty and stderr contains only the error envelope.

## Standalone formats and streams

The following commands reject `--json` before doing work:

- `completion`: its result is a shell script;
- `docs command-map`: its result is generated Markdown;
- `client render xray` and `server render xray`: their result is an Xray JSON file;
- `diag`, `client run`, `server run`, and service `run`: foreground lifecycle processes;
- `server deploy`: a deployment listener;
- `ping`: continuous output needs an NDJSON contract.

The rejection uses the normal JSON error contract. These exceptions do not apply to
ordinary status, list, show, or mutation commands.

## Automation examples

Read the version without parsing display text:

```sh
xp2p --json --version | jq -r '.result.version'
```

Check an error while retaining the process exit code:

```sh
if ! xp2p --json client list >result.json 2>error.json; then
  jq -r '.error.code' error.json
fi
```

The generated files under `commands_map/` are the authoritative inventory of the
real Cobra tree. Every executable leaf records `Machine output: json` or a classified
exception with its reason.
