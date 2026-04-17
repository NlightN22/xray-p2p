# Apply Flow

This document describes how xp2p applies configuration changes via a strict
service-owned apply mechanism. The goal is to make config updates atomic,
audited, and safe to apply from services without relying on CLI flags.

## Key Files and Directories

- `CONFIG_ROOT/.state/`
- `CONFIG_ROOT/.state/live/`
- `CONFIG_ROOT/.state/lkg/`
- `CONFIG_ROOT/.state/apply.request`
- `CONFIG_ROOT/.state/apply.error`
- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`
- `CONFIG_ROOT/config-client/`
- `CONFIG_ROOT/config-server/`

The `apply.request` file is the trigger that asks the service layer to compile
and apply Desired inputs.

## Actors

- CLI, UI, and manual edits: update Desired inputs and create `apply.request`.
- Service layer (`xp2p run` or system service): reads `apply.request`,
  compiles Desired inputs into runtime artifacts, and cleans up.

## High-Level Flow

1. Update Desired inputs (`xp2p-*.toml` and optional JSON snippets).
2. Write `apply.request`.
3. Service detects request and compiles runtime configuration.
4. Service clears `apply.request` on success, or writes `apply.error` on failure.
5. Runtime behavior updates OS routes and TUN state (service layer only).

## Desired Inputs

Desired inputs are always user-editable and live at stable paths:

- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`
- `CONFIG_ROOT/config-client/*.json` (optional snippets)
- `CONFIG_ROOT/config-server/*.json` (optional snippets)

xp2p reads these inputs and compiles them into a final Xray configuration used by the runtime.

For recommended snippet filenames and routing rule insertion points, see `docs/08-config-compilation.md`.

## Read Rules and Exceptions

- Runtime behavior (service run, diagnostics, ping, OS routing) reads live runtime
  artifacts only and never reads Desired inputs directly.
- If Desired inputs change, an apply must be requested via `apply.request`.
- Deploy validation may start a temporary xray-core using a compiled config derived
  from Desired inputs, but it must not write to live or bypass apply.

## Edit + Rollback Flow

This section describes the flow for manual edits, CLI edits, and rollback
using a clear split between Desired, Live, and LKG. Apply requests are tracked
via a marker file.

### Directory Roles

- Desired: user-editable config inputs
  - `CONFIG_ROOT/xp2p-*.toml`
  - `config-client/*.json`, `config-server/*.json`
- Live: active runtime config
  - `CONFIG_ROOT/.state/live/`
- LKG: last known good snapshot (hidden)
  - `CONFIG_ROOT/.state/lkg/`

Live and LKG store compiled runtime artifacts (for example `xray.json`) together with apply metadata.

### Manual Edit Flow

1. User edits Desired files under `CONFIG_ROOT/` or `config-*/`.
2. Watchers debounce bursts of writes.
3. `apply.request` is created after edits settle to trigger service apply.
4. Service compiles Desired inputs and writes live runtime artifacts atomically.
5. On success, the previous live artifact set is stored as LKG (optional).

### CLI Edit Flow

1. CLI writes updates into Desired.
2. `apply.request` is created to trigger service apply.
3. Service compiles Desired inputs into live runtime artifacts.
4. On success, the previous live artifact set is stored as LKG (optional).

### Rollback Flow

1. Apply fails (service/xray/health checks).
2. Service restores live runtime artifacts from LKG (when available).
3. `apply.error` is written with the request ID and failure reason.
4. `apply.request` remains so operators can see the requested change, but the service
   skips repeated apply attempts for the same request ID.
5. Service restarts using restored live artifacts and logs the failure.

## Deploy Flow

Deploy flow details (including apply requests, temporary tunnel validation,
and service start requirements) live in `docs/07-deploy-flow.md` to avoid
duplication.

## Apply Request

The apply trigger file is created at:

- `CONFIG_ROOT/.state/apply.request`
- `CONFIG_ROOT/.state/apply.error`

It includes a role (`client` or `server`) and a request ID. The service
process watches for this file and treats it as the single source of truth
for apply work. When apply fails, the service writes `apply.error` with
the same request ID and failure reason.

## Service Apply

On service start (or restart), the service:

1. Reads `apply.request`.
2. Compiles Desired inputs into live runtime artifacts.
3. Removes `apply.request`.
4. Writes LKG metadata on success (optional).

If apply fails, the service logs the error and keeps `apply.request` so the
operator can investigate or retry. The service will not retry the same
request ID once `apply.error` is recorded; a new apply request must be
created after fixing Desired inputs.

## Routes and OS Changes

OS changes are applied only by the service layer:

- TUN creation and IP assignment.
- Routes and full-tunnel changes.
- DNS overrides (when enabled).

CLI commands and UI flows update Desired inputs and request apply. They do not touch OS-level state directly.

## Runtime OS State Contract (TUN / routes / DNS)

This section defines the runtime contract for service-owned OS state to avoid visible flapping during restarts.

### Ownership and Scope

- The service layer owns OS state (TUN, routes, DNS).
- The service layer must keep OS state consistent with the current Desired runtime mode.
- CLI/UI/manual edits must not directly modify OS state.

### Mode-Driven Transitions

OS state transitions are driven by mode transitions, not by internal restarts:

- Enter full-tunnel (`client.tun_enabled=true` and `client.tun_mode=full`):
  - Replace default routes with the TUN interface.
  - Add bypass routes to all configured endpoints.
  - Apply DNS override to `client.dns_servers` (when configured).
  - Keep full-tunnel active while Desired remains in full-tunnel mode.
- Exit full-tunnel (Desired changes away from full-tunnel):
  - Restore baseline default routes and remove bypass routes.
  - Restore baseline DNS.
- Service stop/uninstall:
  - Restore baseline routes/DNS (best effort) before exiting.

### Restart and Cancellation Semantics

Service restarts caused by `apply.request`, file watchers, health checks, or crash recovery must not cause
route/DNS rollback if Desired remains in full-tunnel mode.

- A child-run cancellation (graceful restart) is not a mode change.
- Rollback/restore is allowed only on explicit stop, explicit mode switch, or hard failures that require leaving the mode.

## Mode Switching

Mode changes (split/full):

- Update `tun_enabled`, `tun_mode`, and `full_tunnel_tag` in Desired TOML.
- Write `apply.request`.
- Service compiles config and restarts runtime as needed.
- In full mode, repeated route re-apply does not rewrite config; it only
  updates OS routes.

## Common Failure Modes

- Invalid Desired TOML / invalid JSON snippets.
- Merge collisions (reserved tags, invalid rule order, conflicts).
- Service not running or apply request not detected.
