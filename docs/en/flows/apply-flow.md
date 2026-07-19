# Apply Flow

All writers in this flow follow the role-scoped
[Desired-to-Live commit protocol](desired-live-commit-protocol.md). That protocol
defines serialization, request acknowledgement, stale-source detection, and
recovery; this page describes the user-visible apply behavior.

This document describes how xp2p applies configuration changes through two
coordinated paths:

- service-owned apply for manual edits, deploy/import, service-layer changes,
  and changes that cannot be updated through the pinned Xray gRPC API;
- runtime-capable CLI apply for Xray resources that can be changed and verified
  through the pinned Xray gRPC API.

The goal is to make config updates atomic, audited, and safe to apply from
services without relying on CLI flags.

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
and apply Desired inputs. Runtime-capable CLI commands do not create
`apply.request` when they successfully apply through the Xray API or when they
only stage Desired inputs while the service is stopped.

## Actors

- Runtime-capable CLI commands: build a candidate config before changing
  Desired inputs, apply and verify it through the Xray API when a running Live
  runtime is available, then publish Live artifacts and persist Desired inputs.
- Manual edits, deploy/import, UI flows, and service-layer changes: update
  Desired inputs and create `apply.request`.
- Service layer (`xp2p run` or system service): reads `apply.request`, detects
  staged Desired inputs on startup, compiles Desired inputs into runtime
  artifacts, and cleans up.

## High-Level Flow

### Service Apply Path

1. Update Desired inputs (`xp2p-*.toml` and optional JSON snippets).
2. Write `apply.request`.
3. Service detects request and compiles runtime configuration.
4. Service clears `apply.request` on success, or writes `apply.error` on failure.
5. Runtime behavior updates OS routes and TUN state (service layer only).

### Runtime-Capable CLI Path

1. Load current Desired inputs and build a candidate config in memory.
2. Validate and compile the candidate without writing Desired or Live.
3. If running Xray is available, apply the candidate through the Xray gRPC API.
4. Verify the running Xray state.
5. On success, publish matching Live artifacts and persist the corresponding
   Desired inputs.
6. If the service is stopped or no running Live runtime is available, persist
   Desired inputs only. The next service/run start compiles and runs them.
7. If the service appears to be running but API apply or verification fails,
   return an error and leave Desired and Live unchanged.

If a runtime-capable redirect change also has an immediate OS route side effect,
the command applies that route change on-flow after the Xray API apply succeeds
and the matching Desired/Live artifacts are persisted. The command must not
create `apply.request` or restart xray-core only to update split routes. If no
running Live runtime is available, no route is changed immediately; the staged
Desired inputs are compiled and applied by the next service/run start.

The service manager state is not the same as runtime availability. A manual
`xp2p run` process can provide a running Live Xray runtime while the OS service
manager reports the service as stopped. Runtime-capable CLI commands must try
the Live runtime/API path first and use the service manager state only to decide
whether an unavailable API is a staged-while-stopped case or a failed running
service case.

## Desired Inputs

Desired inputs are always user-editable and live at stable paths:

- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`
- `CONFIG_ROOT/config-client/*.json` (optional snippets)
- `CONFIG_ROOT/config-server/*.json` (optional snippets)

xp2p reads these inputs and compiles them into a final Xray configuration used by the runtime.

For recommended snippet filenames and routing rule insertion points, see [Config compilation](config-compilation.md).

## Read Rules and Exceptions

- Runtime behavior (service run, diagnostics, ping, OS routing) reads live runtime
  artifacts only and never reads Desired inputs directly.
- Manual edits and service-layer changes request apply via `apply.request`.
- Runtime-capable CLI commands may stage Desired inputs while no running Live
  runtime is available, or update Live after a verified API apply while a
  running Live runtime is available, without creating `apply.request`.
- Deploy validation may start a temporary xray-core using a compiled config derived
  from Desired inputs, but it must not write to live or bypass apply.

## Edit + Rollback Flow

This section describes the flow for manual edits, runtime-capable CLI edits,
service-owned apply, and rollback using a clear split between Desired, Live,
and LKG. Service apply requests are tracked via a marker file.

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

### Runtime-Capable CLI Edit Flow

1. CLI builds and validates a candidate config from current Desired inputs.
2. If a running Live runtime is available, CLI applies the candidate through the
   Xray API and verifies the runtime result.
3. On success, CLI writes Live artifacts that match the running Xray state and
   persists the corresponding Desired inputs.
4. If no running Live runtime is available and the service manager reports the
   service stopped, CLI persists Desired inputs only and leaves Live untouched.
5. If runtime apply or verification fails, CLI returns an error and leaves
   Desired and Live unchanged.

### Service-Owned CLI And UI Flow

1. CLI or UI writes updates into Desired for changes that are not runtime-capable.
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
and service start requirements) live in [Deploy flow](deploy-flow.md) to avoid duplication.

## Apply Request

The apply trigger file is created at:

- `CONFIG_ROOT/.state/apply.request`
- `CONFIG_ROOT/.state/apply.error`

It includes a role (`client` or `server`) and a request ID. The service
process watches for this file and treats it as the single source of truth
for apply work. When apply fails, the service writes `apply.error` with
the same request ID and failure reason.

## Service Apply

On service/run start (or restart), the service layer:

1. Reads `apply.request`, or creates one internally when Desired inputs are
   newer than Live artifacts.
2. Compiles Desired inputs into live runtime artifacts.
3. Removes `apply.request`.
4. Writes LKG metadata on success (optional).

If apply fails, the service logs the error and keeps `apply.request` so the
operator can investigate or retry. The service will not retry the same
request ID once `apply.error` is recorded; a new apply request must be
created after fixing Desired inputs.

### Startup-Only Staged Changes

Some service startup migrations may update Desired inputs before xray-core is
running. These migrations must stage the updated Desired inputs and write
`apply.request`. They must not use the runtime-capable CLI path before
xray-core exists.

After staging, the normal service apply path compiles Desired into Live
artifacts, clears `apply.request` on success, and starts xray-core from the
fresh Live artifacts. This avoids a false runtime-apply failure during service
bootstrap and keeps Desired, Live, and the running process aligned.

Credential rotation is not a startup migration. It changes authentication
between nodes and must use the coordinated rotation flow with client
verification and acknowledgement. A package upgrade must not rotate an active
credential merely because the new service version started.

## Routes and OS Changes

OS changes are normally applied by the service layer:

- TUN creation and IP assignment.
- Routes and full-tunnel changes.
- DNS overrides (when enabled).

CLI commands and UI flows do not touch OS-level state directly for
service-owned changes. Changes that affect service-owned OS state update
Desired inputs and request service apply. Runtime-capable CLI commands may
update Xray runtime resources through the API.

Redirect CIDR changes are the exception for immediate route side effects. When
the running Live Xray runtime is available and the routing rule is applied and
verified through the API, the same command reconciles the matching OS split
route on-flow. This keeps Xray routing, Live artifacts, Desired inputs, and OS
routes aligned without an `apply.request` restart. TUN lifecycle, DNS, firewall,
and nftables remain service-owned.

## Runtime OS State Contract (TUN / routes / DNS)

This section defines the runtime contract for service-owned OS state to avoid visible flapping during restarts.

### Ownership and Scope

- The service layer owns OS state (TUN, routes, DNS), except for immediate
  split-route reconciliation performed by a successful runtime-capable redirect
  command.
- The service layer must keep OS state consistent with the current Desired runtime mode.
- CLI/UI/manual edits must not directly modify service-owned OS state.

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

### Pending State and Retry (Windows)

On Windows, TUN readiness can be delayed or unstable across restarts (adapter disconnected, IPv4 missing, DAD not preferred).
When Desired is full-tunnel but the adapter is not ready, runtime enters a pending state instead of rolling back OS state.

- Pending state is recorded in `CONFIG_ROOT/xp2p-client.tun-full.json` as `phase = "full_pending"` with a stable `pending_reason`.
- While pending, routes and DNS override are not applied.
- The service retries through restarts using exponential backoff (2s, 4s, 8s, ... capped at 30s) until the adapter is `up`/`preferred`.

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
- Runtime API apply or verification failure for a runtime-capable CLI command.
