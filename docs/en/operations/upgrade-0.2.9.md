# Upgrade to 0.2.9

Version 0.2.9 adds explicit `xp2p-diag` health capability discovery for
external Xray and 3x-ui endpoints.

Existing Desired configuration requires no changes. The
`required`, `auto`, and `disabled` heartbeat modes retain their meanings.
Persisted legacy `capability: "detected"` records remain readable and are
treated as the full `xp2p-heartbeat` check. New state may store
`xp2p-heartbeat` or `xp2p-diag` explicitly.

Automation can read the complete status vocabulary and thresholds from
`xp2p heartbeat contract`. The initial machine-readable contract version is
`1`.

Legacy flat tunnel fields also remain compatibility-normalized in 0.2.9. Their
planned rejection is deferred to 0.3.0, so upgrading to this release does not
require rewriting existing Desired inputs.

The network lifecycle changes do not alter configuration files, CLI contracts,
service invocation defaults, or persisted state layouts. Existing clients
remain protocol-compatible with upgraded servers, which bound abandoned idle
connections from older clients. Upgrade clients as well to apply the complete
client-side ownership and reuse fix. No manual resource cleanup or
configuration migration is required.

Upgrade both the client and the standalone sidecar to 0.2.9 to enable
ping-only health. Older sidecars do not advertise the capability, so the client
continues to require the full report and reports its `404` as
`FAILURE_STAGE=report`.

For strict TLS deployments, mount the certificate and key for the endpoint
DNS name into the sidecar and restart it after certificate renewal.
