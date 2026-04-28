# Deploy Flow

This document describes the deploy flow for xp2p and how it interacts with
configuration compilation and service startup.

## Scope

- Applies to both client and server deploy.
- Follows the Apply Flow in [apply-flow.md](apply-flow.md).

## Key Rules

- Deploy updates Desired inputs and writes `apply.request`.
- Deploy does not start services.
- Deploy may start a temporary xray-core instance to validate the tunnel.
- Services are started explicitly by the operator after deploy succeeds.
- Service startup compiles Desired inputs into live runtime artifacts before running.

## Deploy Overview

1. Client deploy generates a deploy link and listens for the server.
2. Server deploy receives the encrypted manifest and updates Desired inputs:
   - `CONFIG_ROOT/xp2p-server.toml`
   - Optional JSON snippets under `CONFIG_ROOT/config-server/` (when used by the deploy spec)
3. Server deploy writes `apply.request` for the server role.
4. Client deploy receives the server response and updates Desired inputs:
   - `CONFIG_ROOT/xp2p-client.toml`
   - Optional JSON snippets under `CONFIG_ROOT/config-client/` (when used by the deploy spec)
5. Client deploy writes `apply.request` for the client role.

## Temporary Tunnel Validation

Deploy may start xray-core with a compiled config to validate connectivity:

- This is a temporary runtime used only during deploy.
- It does not write live runtime artifacts.
- It does not start the system service.
- It is shut down when deploy finishes.

If service is already running, deploy must not stop or restart it. Deploy must
still validate the tunnel and must not rely on the service runtime for that
validation. After a successful deploy, the operator must restart the service
to apply the requested changes. If the service is not running, start it instead.

## Deploy With Live Config And Running Service

Deploy can run against an existing installation with live config and an active
service:

- Desired inputs remain the source of truth for deploy updates.
- Deploy updates Desired inputs and writes `apply.request`.
- The running service must keep working and must not be restarted by deploy.
- The temporary deploy xray-core is used only for tunnel validation and must
  not overwrite or reuse the service runtime.
- After a successful deploy:
  - If the service is running, restart it to apply the requested changes.
  - If the service is not running, start it to apply the requested changes.

## Service Startup After Deploy

After a successful deploy, the operator starts services explicitly:

- `xp2p server service start`
- `xp2p client service start`

On startup, the service:

1. Loads `apply.request`.
2. Compiles Desired inputs into live runtime artifacts.
3. Removes `apply.request`.
4. Starts the runtime using live config.

If there is no compiled live runtime config, the service exits with a
"no config available" status without starting xray-core.

## Failure Signals

Common deploy issues are:

- Invalid Desired TOML / invalid JSON snippets.
- Merge collisions (reserved tags, invalid rule order, conflicts).
- Service not started after a successful deploy.

If deploy logs report invalid Desired inputs or merge collisions, fix the Desired
files and request apply again.
