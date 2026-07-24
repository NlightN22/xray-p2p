---
name: xray-p2p-cli-output-check
description: Audit and verify the xray-p2p public CLI JSON output contract. Use when adding, removing, or changing Cobra commands, flags, aliases, help or command metadata, stdout/stderr behavior, output models, status/list/show/mutation results, JSON or NDJSON contracts, or generated command maps.
---

# Check XP2P CLI output

Work from the repository root.

1. Inspect the diff and identify affected command paths.
2. Build the real tree with `root.NewCommand`; do not infer commands only from files.
3. Compare every executable leaf with the explicit inventory in
   `go/cmd/xp2p/root/output_inventory.go`. Fail for missing and stale entries. Never
   assign a default class to an unregistered command.
4. Require an explicit reason for every non-JSON class. Do not invent a new exception
   for an ordinary status, list, show, or mutation command.
5. Check that every JSON command publishes a typed result. Reject captured human
   output, `result.text`, table parsing, and rendering-then-decoding adapters.
6. Check that `--json` produces one newline-terminated document on stdout and that
   `encoding/json` decodes it without cleanup.
7. Check result field types, empty results, deterministic ordering where meaningful,
   UTC RFC 3339 timestamps, and documented duration and byte representations.
8. Check credentials: retain required secrets for create, rotate, provision, export,
   and explicit-show commands; reject incidental secrets in status, list, health,
   diagnostics, warnings, logs, and errors.
9. Exercise success, error, empty-result, number/boolean, credential, Unicode/control
   character, warning, prompt, and ANSI cases for every affected leaf.
10. Require failures to have a non-zero exit code, empty stdout,
   and one structured error document on stderr without Cobra usage or logs.
11. Search repository consumers for `splitlines`, regular expressions, table parsing,
    and credential-line extraction. Migrate machine consumers to JSON.
12. Verify human output is unchanged unless the task explicitly changes it.
13. Run targeted tests for the changed packages, including:

```powershell
go test ./go/internal/cli/output ./go/internal/cli/commandmap ./go/cmd/xp2p/root
```

14. Run `make command-map` after command, flag, alias, help, metadata, or output
    classification changes. Review the generated diff for the concrete command paths.
15. For any Go change, use the repository Go test workflow:

```powershell
make test
make test-wsl
```

Report the affected command paths, violated contracts, generated files, and exact test
results. Do not automatically change the public contract or add exceptions without an
explicit implementation decision.
