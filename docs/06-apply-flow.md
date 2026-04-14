# Apply Flow

This document describes how xp2p applies configuration changes using the
pending/apply mechanism. The goal is to make config updates atomic, audited,
and safe to apply from services without relying on CLI flags.

## Key Files and Directories

- `CONFIG_ROOT/.state/`
- `CONFIG_ROOT/.state/pending/`
- `CONFIG_ROOT/.state/live/`
- `CONFIG_ROOT/.state/lkg/`
- `CONFIG_ROOT/.state/apply.request`
- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`
- `config-client/`
- `config-server/`

Pending snapshots live under `CONFIG_ROOT/.state/pending`. The `apply.request`
file is the trigger that asks the service layer to apply pending changes.

## Actors

- CLI and UI: write pending config updates and create `apply.request`.
- Service layer (`xp2p run` or system service): reads `apply.request`,
  applies pending changes, and cleans up.

## High-Level Flow

1. Update configuration in pending mode.
2. Write `apply.request`.
3. Service detects request and applies pending config.
4. Service clears `apply.request` and pending artifacts.
5. Runtime behavior updates OS routes and TUN state (service layer only).

## Pending Updates

When a command updates configuration:

- The live config is used as a base.
- The update is written to the pending config under
  `CONFIG_ROOT/.state/pending/`.
- The pending config must include the full `client.xray` or `server.xray`
  sections, not just the changed fields.
- Pending is the source of truth for follow-up edits: if a pending config
  exists, subsequent commands must read and update the pending snapshot
  instead of mixing in live data. Live config is used only to seed pending
  when no pending snapshot exists.

This ensures that apply can generate all required runtime files (inbounds,
outbounds, routing, logs) from a complete config snapshot.

## Read Rules and Exceptions

- Pending is authoritative only for staging edits and apply.
- Runtime behavior (service run, diagnostics, ping, OS routing) reads live
  config only, never pending.
- If pending exists without live, the runtime should request apply
  (`apply.request`) and wait for service apply instead of running from pending.
- Limited exceptions are allowed:
  - Asset presence checks may treat pending as "installed" to avoid triggering
    `--auto-install` when a full pending snapshot already exists.
  - Deploy validation may start temporary xray-core using the pending snapshot,
    but it must not write to live or bypass apply.

## Edit + Rollback Flow

This section describes the flow for manual edits, CLI edits, and rollback
using a clear split between Desired, Pending, Live, and LKG.

### Directory Roles

- Desired: user-editable config inputs
  - `CONFIG_ROOT/xp2p-*.toml`
  - `config-client/*.json`, `config-server/*.json`
- Pending: apply snapshot
  - `CONFIG_ROOT/.state/pending/`
- Live: active runtime config
  - `CONFIG_ROOT/.state/live/`
- LKG: last known good snapshot (hidden)
  - `CONFIG_ROOT/.state/lkg/`

Pending, Live, and LKG keep a mirrored structure that includes:

- `xp2p-client.toml`, `xp2p-server.toml`
- `config-client/*.json`, `config-server/*.json`

### Manual Edit Flow

1. User edits Desired files under `CONFIG_ROOT/` or `config-*/`.
2. Watchers debounce bursts of writes and then capture a complete snapshot
   into Pending.
3. Snapshot writes are atomic (write temp files, then rename into place) to
   avoid partial pending state.
4. `apply.request` is created after the snapshot is fully written to trigger
   service apply.
5. Service applies Pending to Live and clears Pending.
6. On success, the full Live snapshot is written to LKG.

### CLI Edit Flow

1. CLI writes updates into Pending (or into Desired, then Pending).
2. `apply.request` is created to trigger service apply.
3. Service applies Pending to Live and clears Pending.
4. On success, the full Live snapshot is written to LKG.

### Rollback Flow

1. Apply fails (service/xray/health checks).
2. Service restores Live from LKG.
3. Pending is cleared and `apply.request` removed or preserved with an error
   marker (policy decision).
4. Service restarts using restored Live and logs the failure.

## Deploy Flow

Deploy flow details (including pending updates, temporary tunnel validation,
and service start requirements) live in `docs/07-deploy-flow.md` to avoid
duplication.

## Apply Request

The apply trigger file is created at:

- `CONFIG_ROOT/.state/apply.request`

It includes a role (`client` or `server`) and a request ID. The service
process watches for this file and treats it as the single source of truth
for pending work.

## Service Apply

On service start (or restart), the service:

1. Reads `apply.request`.
2. Loads the pending config set.
3. Applies the pending config to live config/runtime files.
4. Removes `apply.request`.
5. Cleans up pending artifacts on success.

If apply fails, the service logs the error and keeps `apply.request` so the
operator can investigate or retry.

## Routes and OS Changes

OS changes are applied only by the service layer:

- TUN creation and IP assignment.
- Routes and full-tunnel changes.
- DNS overrides (when enabled).

CLI commands and UI flows only prepare pending configuration and request
apply. They do not touch OS-level state directly.

## Mode Switching

Mode changes (split/full):

- Update `tun_enabled`, `tun_mode`, and `full_tunnel_tag` in pending config.
- Write `apply.request`.
- Service applies pending config and restarts runtime as needed.
- In full mode, repeated route re-apply does not rewrite config; it only
  updates OS routes.

## Common Failure Modes

- Missing `client.xray` or `server.xray` in pending config.
- Missing or unreadable pending config files.
- Service not running or apply request not detected.

If a test reports `client xray config not found in .../pending/xp2p-client.toml`,
the pending config was created without a complete `client.xray` section.
