# Deploy Flow

This document describes the deploy flow for xp2p and how it interacts with
pending configuration and service startup.

## Scope

- Applies to both client and server deploy.
- Follows the Apply Flow in `06-apply-flow.md`.

## Key Rules

- Deploy writes pending config and `apply.request`.
- Deploy does not start services.
- Deploy must start a temporary xray-core instance to validate the tunnel.
- Services are started explicitly by the operator after deploy succeeds.
- Service startup applies pending config to live files before running.

## Deploy Overview

1. Client deploy generates a deploy link and listens for the server.
2. Server deploy receives the encrypted manifest and writes pending config:
   - `CONFIG_ROOT/.apply/pending/xp2p-server.toml` with full `server.xray`.
   - `config-server/.apply/pending/` with `inbounds.json`, `outbounds.json`,
     `routing.json`, `logs.json`, and cert/key files.
3. Server deploy writes `apply.request` for the server role.
4. Client deploy receives the link and writes pending config:
   - `CONFIG_ROOT/.apply/pending/xp2p-client.toml` with full `client.xray`.
   - `config-client/.apply/pending/` with `inbounds.json`, `outbounds.json`,
     `routing.json`, `logs.json`.
5. Client deploy writes `apply.request` for the client role.

## Temporary Tunnel Validation

Deploy must start xray-core with the pending config to validate connectivity:

- This is a temporary runtime used only during deploy.
- It does not apply pending config to live files.
- It does not start the system service.
- It is shut down when deploy finishes.

If service is already running, deploy must not stop or restart it. Deploy must
still validate the tunnel and must not rely on the service runtime for that
validation. After a successful deploy, the operator must restart the service
to apply pending changes. If the service is not running, start it instead.

## Deploy With Live Config And Running Service

Deploy can run against an existing installation with live config and an active
service:

- Pending remains the source of truth for deploy updates.
- Deploy updates pending config and writes `apply.request`.
- The running service must keep working and must not be restarted by deploy.
- The temporary deploy xray-core is used only for tunnel validation and must
  not overwrite or reuse the service runtime.
- After a successful deploy:
  - If the service is running, restart it to apply pending changes.
  - If the service is not running, start it to apply pending changes.

## Service Startup After Deploy

After a successful deploy, the operator starts services explicitly:

- `xp2p server service start`
- `xp2p client service start`

On startup, the service:

1. Loads `apply.request`.
2. Applies the pending config to live files.
3. Removes `apply.request` and clears pending artifacts.
4. Starts the runtime using live config.

If there is neither pending nor live config, the service exits with a
"no config available" status without starting xray-core.

## Failure Signals

Common deploy issues are:

- Missing full `client.xray` or `server.xray` in pending config.
- Missing required pending config files.
- Service not started after a successful deploy.

If deploy logs report missing config files in pending, the deploy step produced
an incomplete snapshot.
