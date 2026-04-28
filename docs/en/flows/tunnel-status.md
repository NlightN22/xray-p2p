# Tunnel Runtime Status Logic

This note documents how tunnel runtime status is derived from the client state file.

## Data source

Runtime status reads `C:\ProgramData\xp2p\xp2p-client.state.json` and uses:

- `tun_enabled`
- `mode`
- `runtime.tun.*`
- `runtime.routes.*`
- `runtime.socks_ready`
- `runtime.last_error`
- `runtime.timestamp`

If the file is missing or `runtime` is empty, the status is `Tun: Unknown`.

## Status derivation

### Tunnel readiness

- If `tun_enabled=false` (proxy mode):
  - Ready when `runtime.socks_ready=true`
  - Pending when `runtime.socks_ready=false`
- If `tun_enabled=true`:
  - `tun_ready = runtime.tun.ready`
  - `routes_ready = runtime.routes.full_applied || runtime.routes.redirect_applied`
  - Ready when both `tun_ready` and `routes_ready` are true
  - Pending otherwise

### Route label

The status logic uses runtime flags with the current mode:

- `Proxy` when `tun_enabled=false`
- `Full` when `runtime.routes.full_applied=true`
- `Split` when `runtime.routes.redirect_applied=true`
- `Tun` when `tun_enabled=true` and neither route flag is true

### Errors

If `runtime.last_error` is present, append it to the detail line.

## Example strings

- `Proxy: Ready (SOCKS)`
- `Tun: Ready | Routes: Full`
- `Tun: Pending | Routes: Split`
- `Tun: Unknown`
