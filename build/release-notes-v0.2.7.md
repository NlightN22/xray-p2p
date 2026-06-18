# Release notes v0.2.7

## Runtime apply through Xray gRPC

- Added direct Xray gRPC clients and generated protocol bindings for routing, handler, inbound, outbound, observatory, and stats APIs.
- Added runtime diff classification for routing, inbound, inbound user, outbound, and mixed configuration changes.
- Runtime diff classification now supports replacing an existing outbound with the same tag and replacing an existing Trojan inbound user with the same email, allowing credential updates to be applied as remove/add operations through Xray gRPC.
- Client and server apply flows can now apply supported Xray changes through the running Xray API and verify the runtime result before persisting Desired/Live state.
- Server user changes now use live Xray API application where supported instead of forcing a full Xray restart.
- `xp2p server user remove` no longer writes an apply request when the requested user and related reverse/redirect state are already absent, avoiding unnecessary service apply/restart work for no-op removals.
- Added smoke coverage for the bundled Xray API integration.

## Runtime state, stats, and observability

- Added direct Xray stats support and surfaced Xray traffic counters in client/server state output.
- Fixed client and server state source handling so runtime views read Live artifacts while staged checks use pending/Desired inputs.
- Added `xp2p client obs` for inspecting Xray observatory output from the client side.
- Added command-map documentation for `xp2p client obs`.

## Diagnostics ping modes

- Added `xp2p ping --continuous` for manual diagnostics that keep sending ping requests until interrupted.
- Added `xp2p ping --keep-open` for TCP diagnostics that keep one connection open and fail when that persistent connection breaks.
- Added an explicit keep-open diagnostics request so the responder switches only requested TCP sessions into persistent mode while normal `PING <nonce>` sessions remain one-request connections.
- Added Go coverage for continuous ping cancellation, keep-open client exchanges, and responder-side keep-open TCP sessions.

## Xray loop protection

- Added Xray guard components that inspect running Xray instances and detect potentially conflicting local Xray processes.
- Added loop-protection coverage so xp2p can avoid routing through an unsafe local Xray loop.
- Added tests for Xray guard detection behavior.

## Enable/disable flows

- Added runtime enable/disable flows for client endpoints, client reverse tunnels, server users, server reverse tunnels, and redirect rules.
- Added active-state helpers so disabled entries stay in Desired configuration but are excluded from generated active runtime configuration.
- Added CLI command coverage for client/server reverse enable/disable and redirect enable/disable.

## Credential update commands

- Added `xp2p client update <hostname|tag>` to change only the selected client endpoint `user` and/or `password` while preserving the endpoint host, outbound tag, redirects, and reverse tunnel bindings.
- Added `xp2p server user update <id>` to change only the selected server Trojan user id and/or password while preserving existing reverse portal tags and redirect bindings.
- Credential updates write Desired inputs first, attempt runtime apply through Xray gRPC immediately, publish matching Live artifacts on success, and leave the apply request in place only when restart fallback is required.
- Added validation so server user updates reject duplicate target user ids and invalid passwords without changing Desired state.

## Connection link workflows

- Added `xp2p client list --link` to print configured client endpoints as ready-to-use `trojan://...` connection links.
- Added `xp2p server user add --link <trojan://...>` to add a server Trojan user from an existing connection link, reusing the link user and password.
- Server user import from a link rejects conflicting explicit `--id`, `--password`, or `--key` values so the link remains the single credential source.
- Regenerated command maps to document the new `--link` flags for client listing and server user creation.

## Redirect command behavior

- `xp2p client redirect` now lists configured redirects by default, matching `xp2p client redirect list`.
- `xp2p server redirect` now lists configured redirects by default, matching `xp2p server redirect list`, and accepts the same list flags (`--path`, `--config-dir`, `--pending`).
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
- The UI mode manager now skips apply requests when applying the already-current client/server mode does not change Desired configuration.
- Updated client and server log templates.
- Added version parsing and version metadata tests.

## Documentation and release tooling

- Added the release automation script used for new release preparation.
- Updated release, aggregate release, Pages deploy, and MkDocs build workflows.
- Added `xp2p docs command-map --dir <path>` to generate compact command maps directly from the Cobra command tree.
- Added `make command-map` to regenerate `commands_map` through WSL so Linux-only commands such as `nat-redirect` and `dns-forward` are included from the Linux build view.
- Regenerated `commands_map` from the CLI source of truth and added explicit default-behavior notes for commands that list entries when called without a subcommand.
- Repository contributor instructions now require `make command-map` after CLI command, flag, help, or command metadata changes.
- Added OpenWrt install-from-Pages guest helper coverage for validating published install scripts.
- Added multilingual documentation layout with English and Russian documentation trees.
- Added Russian documentation for getting started, guides, operations, references, and flow documents.
- Added documentation for the normalization pipeline, service control, shell completion, diagnostics, backup/migration, chain routing, and advanced redirect/routing flows.
- Updated MkDocs configuration, language switch assets, Mermaid initialization, and code-copy/code-wrap styling.

## Test infrastructure

- Split large host test helper modules into focused Linux, OpenWrt, and Windows helper modules.
- Added OpenWrt install-from-pages test coverage.
- Added Linux/OpenWrt host coverage that verifies duplicate client endpoint installs leave Desired configuration unchanged and do not create a new apply request.
- Added Linux host coverage that verifies real server user removal creates an apply request, while repeating the same removal as a no-op does not.
- Updated Linux/OpenWrt host redirect tests to cover removing a redirect by CIDR without an explicit tag/host when exactly one matching redirect exists.
- Added Linux host coverage for server redirect cleanup when removing a user with duplicate-CIDR redirects across different reverse tags.
- Added Go unit coverage for positive and negative redirect binding selection scenarios on client and server commands.
- Added Go unit coverage for server redirect cleanup when removing users and for tag-only cleanup of orphaned duplicate-CIDR redirects.
- Added Go unit coverage to ensure missing user/endpoint/forward/redirect removal paths do not create apply requests when Desired state is unchanged.
- Added Go unit coverage for the shared binding resolver.
- Added Go unit coverage for credentials-only client endpoint updates and server user updates that preserve redirects and reverse bindings.
- Added Go unit coverage for rendering client endpoints as Trojan links and for adding server users from Trojan links.
- Added runtime-apply unit coverage for same-tag outbound replacement and same-email Trojan inbound user replacement.
- Added Linux host coverage for a successful client endpoint credential update that preserves redirect routing, and for a rejected server user update when the new user id already exists.

## Upgrade notes

- Automation that used `xp2p client redirect list` may keep using it; `xp2p client redirect` is now equivalent for listing.
- Automation that used `xp2p server redirect list` may keep using it; `xp2p server redirect` is now equivalent for listing.
- Redirect enable/disable without `--tag` or `--host` no longer treats an omitted binding as an implicit target-wide operation. Use `--all` for an explicit mass operation, or provide/allow selection of a specific matching binding.
- For non-interactive redirect add/remove/enable/disable flows, pass `--tag`, `--host`, or `--quiet`; `--quiet` now fails if the matching binding is ambiguous.
- For `xp2p server redirect add`, use `--user <user>` when selecting a reverse portal by server user id. `--tag <tag>` is still reserved for the reverse outbound tag.
- If a previous version left an orphaned server redirect after user removal, clean it with `xp2p server redirect remove --tag <tag>`.
- Re-running `xp2p client install` with an already configured endpoint now fails without touching Desired state or scheduling apply work. Remove the endpoint first, or pass `--force` to replace it intentionally.
- Use `xp2p client update <hostname|tag>` or `xp2p server user update <id>` when only credentials need to change; these commands intentionally preserve existing tags, redirects, and reverse bindings.
- Use `xp2p client list --link` to recover a configured client connection link, and `xp2p server user add --link <link>` to recreate the matching server user from that link.
