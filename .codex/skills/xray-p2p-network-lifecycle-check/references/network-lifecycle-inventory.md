# Network Lifecycle Inventory

Use this as the current review map, then confirm it against the source and diff.

| Flow | Expected owner and lifecycle | Focused evidence |
|---|---|---|
| Heartbeat, subscription, rotation, acknowledgement, apply probe | The client runner owns reusable per-endpoint control clients, prunes stale endpoints, joins workers, then shuts clients down. | `go/internal/client/*lifecycle_test.go` and resource plateau gate |
| Standalone and continuous ping | One `ping.Run` invocation owns one direct or SOCKS client across all iterations and shuts it down on every return. | `go/internal/diagnostics/ping/*lifecycle_test.go` |
| HA prepare and commit | One coordinator operation owns secure and insecure peer clients and reuses them across both phases. | HA unit and control-plane tests |
| Identity and SCIM sync | One scheduler or snapshot owns the TLS-policy client shared by all SCIM requests; scheduler stop joins active sync before client shutdown. | `go/internal/identitysync/*lifecycle_test.go` and scheduler tests |
| Xray asset sync | One sync operation owns one client shared by all downloads unless a caller injects a `Doer`. | Xray asset sync tests |
| Control and diagnostics HTTP servers | The returned server owner stops admission, cancels and joins work, and performs bounded shutdown through `go/internal/nethttp`. | `go/internal/nethttp` and server lifecycle tests |
| Deploy TCP server and session | Listener owner closes admission and active connections and joins handlers; client session is explicitly and idempotently closable. | CLI server deploy lifecycle tests |
| Xray gRPC apply | Each runtime-apply operation owns and closes its bounded gRPC client. | `go/internal/xraylive` and API tests |
| Public host detection | Package provider owns a process-lifetime client sharing the default transport under a documented analyzer exception. | `go/internal/netutil/public_host_test.go` |
| External subscription fetch | Per-fetch client value shares an injected or default transport and only owns redirect and timeout policy. | `go/internal/subscription/http_source_test.go` |

Production HTTP construction belongs in `go/internal/nethttp`. Review any
remaining exception through its adjacent
`nethttp-lifecycle:allow <rule> owner=<symbol> lifetime=<scope> reason=<text>`
directive. Periodic control-plane changes require `make resource-plateau`;
release evidence requires `make resource-plateau-nightly`.
