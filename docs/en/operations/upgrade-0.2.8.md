# Upgrade to 0.2.8

No manual service stop or configuration repair is required for a normal
OpenWrt package upgrade.

The upgrade fixes three recovery and compatibility problems:

- OpenWrt package replacement stops running roles without disabling them or
  deleting files installed by the new package.
- A failed apply generation from an older xp2p compiler no longer blocks the
  current version from rebuilding Live artifacts from Desired inputs.
- Service startup no longer rotates legacy credentials. Credential rotation
  remains an explicit coordinated operation.

When an older Live runtime and its matching `apply.error` are present, service
startup records a new apply generation, compiles Desired with the current
binary, and publishes current Live metadata. Failures from the current
compiler remain blocked until Desired changes, preserving the existing retry
protection.

Heartbeat state now stores the latest health result explicitly. Failed checks
use their real observation time instead of a synthetic timestamp one hour in
the past. Existing heartbeat state without the new field remains readable.
