# Apply Flow

This document describes how xp2p applies configuration changes using the
pending/apply mechanism. The goal is to make config updates atomic, audited,
and safe to apply from services without relying on CLI flags.

## Key Files and Directories

- `CONFIG_ROOT/.apply/`
- `CONFIG_ROOT/.apply/pending/`
- `CONFIG_ROOT/.apply/apply.request`
- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`

Pending config artifacts live under `CONFIG_ROOT/.apply/pending`. The
`apply.request` file is the trigger that asks the service layer to apply
pending changes.

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
  `CONFIG_ROOT/.apply/pending`.
- The pending config must include the full `client.xray` or `server.xray`
  sections, not just the changed fields.
- Pending is the source of truth for follow-up edits: if a pending config
  exists, subsequent commands must read and update the pending snapshot
  instead of mixing in live data. Live config is used only to seed pending
  when no pending snapshot exists.

This ensures that apply can generate all required runtime files (inbounds,
outbounds, routing, logs) from a complete config snapshot.

## Deploy Flow (Client + Server)

This section describes the expected deploy sequence and where pending data
is written.

1. Client deploy generates a deploy link (`trojan://...`) and waits for the
   server to connect.
2. Server deploy receives the encrypted manifest, validates it, and prepares
   the server-side pending config:
   - `CONFIG_ROOT/.apply/pending/xp2p-server.toml` (full `server.xray`)
   - `config-server/.apply/pending/` with `inbounds.json`, `outbounds.json`,
     `routing.json`, `logs.json`, and cert/key files
3. Server deploy adds or updates the user in the pending config and writes
   `apply.request` with role `server`.
4. Client deploy receives the link and installs locally into pending:
   - `CONFIG_ROOT/.apply/pending/xp2p-client.toml` (full `client.xray`)
   - `config-client/.apply/pending/` with `inbounds.json`, `outbounds.json`,
     `routing.json`, `logs.json`
5. Client deploy writes `apply.request` with role `client`.
6. If the client was already installed, the deploy flow updates the existing
   endpoint in pending instead of creating a new install root.
7. Deploy does not start services. During deploy, xp2p may start a temporary
   xray-core instance using the pending config to validate the tunnel and
   connectivity. This is part of the deploy process only.
8. After a successful deploy, the operator starts services explicitly (for
   example, `xp2p client service start` and `xp2p server service start`).
9. Service layer applies pending config to live files, removes
   `apply.request`, and clears pending artifacts on success.

## Apply Request

The apply trigger file is created at:

- `CONFIG_ROOT/.apply/apply.request`

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
