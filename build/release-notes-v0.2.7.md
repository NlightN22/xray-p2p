# Release notes v0.2.7

## Runtime apply through Xray gRPC

- Added direct Xray gRPC clients and generated protocol bindings for routing, handler, inbound, outbound, observatory, and stats APIs.
- Added runtime diff classification for routing, inbound, inbound user, outbound, and mixed configuration changes.
- Client and server apply flows can now apply supported Xray changes through the running Xray API and verify the runtime result before persisting Desired/Live state.
- Server user changes now use live Xray API application where supported instead of forcing a full Xray restart.
- Added smoke coverage for the bundled Xray API integration.

## Runtime state, stats, and observability

- Added direct Xray stats support and surfaced Xray traffic counters in client/server state output.
- Fixed client and server state source handling so runtime views read Live artifacts while staged checks use pending/Desired inputs.
- Added `xp2p client obs` for inspecting Xray observatory output from the client side.
- Added command-map documentation for `xp2p client obs`.

## Xray loop protection

- Added Xray guard components that inspect running Xray instances and detect potentially conflicting local Xray processes.
- Added loop-protection coverage so xp2p can avoid routing through an unsafe local Xray loop.
- Added tests for Xray guard detection behavior.

## Enable/disable flows

- Added runtime enable/disable flows for client endpoints, client reverse tunnels, server users, server reverse tunnels, and redirect rules.
- Added active-state helpers so disabled entries stay in Desired configuration but are excluded from generated active runtime configuration.
- Added CLI command coverage for client/server reverse enable/disable and redirect enable/disable.

## Redirect command behavior

- `xp2p client redirect` now lists configured redirects by default, matching `xp2p client redirect list`.
- Updated Linux, OpenWrt, and Windows host tests to use the shorter `xp2p client redirect` form where redirects are listed.
- `xp2p server user remove` now removes server redirect rules tied to the removed user's reverse outbound tag, preventing orphaned redirects after Desired/apply-based changes.
- `xp2p server redirect remove --tag <tag>` can now be used as a recovery cleanup path for orphaned server redirects when the CIDR/domain selector is unavailable or ambiguous.
- `xp2p server redirect add` now supports `--user <user>` to select the reverse portal by server user id while keeping `--tag` limited to real reverse outbound tags.
- Redirect add/remove/enable/disable now resolve `tag`/`host` consistently:
  - one matching binding is selected automatically without prompting;
  - multiple matching bindings prompt the user;
  - `--quiet` fails on ambiguous binding selection instead of choosing implicitly;
  - `--all` remains the explicit mass-operation mode for enable/disable.
- `xp2p client mode tun full` now uses the same binding resolver when selecting the full-tunnel endpoint.

## DNS forward normalization

- Added canonical DNS forward state handling and compatibility normalization for older state shapes.
- Updated DNS forward add/list/remove behavior to work through normalized state.
- Added DNS forward normalization tests.

## Configuration and logging

- Added apply request support for client and server flows that need service/run apply processing.
- Added runtime service runner plumbing used by apply-driven workflows.
- `xp2p client install` now rejects duplicate client endpoints by `hostname:port` before rewriting Desired configuration or creating a new apply request. Use `--force` when an existing endpoint must be replaced.
- Updated client and server log templates.
- Added version parsing and version metadata tests.

## Documentation and release tooling

- Added the release automation script used for new release preparation.
- Updated release, aggregate release, Pages deploy, and MkDocs build workflows.
- Added OpenWrt install-from-Pages guest helper coverage for validating published install scripts.
- Added multilingual documentation layout with English and Russian documentation trees.
- Added Russian documentation for getting started, guides, operations, references, and flow documents.
- Added documentation for the normalization pipeline, service control, shell completion, diagnostics, backup/migration, chain routing, and advanced redirect/routing flows.
- Updated MkDocs configuration, language switch assets, Mermaid initialization, and code-copy/code-wrap styling.

## Test infrastructure

- Split large host test helper modules into focused Linux, OpenWrt, and Windows helper modules.
- Added OpenWrt install-from-pages test coverage.
- Added Linux/OpenWrt host coverage that verifies duplicate client endpoint installs leave Desired configuration unchanged and do not create a new apply request.
- Updated Linux/OpenWrt host redirect tests to cover removing a redirect by CIDR without an explicit tag/host when exactly one matching redirect exists.
- Added Linux host coverage for server redirect cleanup when removing a user with duplicate-CIDR redirects across different reverse tags.
- Added Go unit coverage for positive and negative redirect binding selection scenarios on client and server commands.
- Added Go unit coverage for server redirect cleanup when removing users and for tag-only cleanup of orphaned duplicate-CIDR redirects.
- Added Go unit coverage for the shared binding resolver.

## Upgrade notes

- Automation that used `xp2p client redirect list` may keep using it; `xp2p client redirect` is now equivalent for listing.
- Redirect enable/disable without `--tag` or `--host` no longer treats an omitted binding as an implicit target-wide operation. Use `--all` for an explicit mass operation, or provide/allow selection of a specific matching binding.
- For non-interactive redirect add/remove/enable/disable flows, pass `--tag`, `--host`, or `--quiet`; `--quiet` now fails if the matching binding is ambiguous.
- For `xp2p server redirect add`, use `--user <user>` when selecting a reverse portal by server user id. `--tag <tag>` is still reserved for the reverse outbound tag.
- If a previous version left an orphaned server redirect after user removal, clean it with `xp2p server redirect remove --tag <tag>`.
- Re-running `xp2p client install` with an already configured endpoint now fails without touching Desired state or scheduling apply work. Remove the endpoint first, or pass `--force` to replace it intentionally.
