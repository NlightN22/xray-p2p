---
name: xray-p2p-network-lifecycle-check
description: Audit and verify ownership, reuse, pruning, response handling, shutdown, recovery, and resource plateau behavior for xray-p2p network resources. Use when changing or reviewing HTTP or gRPC clients and servers, transports, dialers, listeners, connection pools, endpoint handlers, heartbeat, subscription, credential rotation, HA sync, identity or SCIM sync, ping, background network loops, retries, timeouts, cancellation, shutdown, or network recovery.
---

# Xray P2P Network Lifecycle Check

Audit the diff mechanically and identify the owner and bounded lifetime of every
affected network resource. A request timeout is not a substitute for resource
ownership, idle pruning, or shutdown.

## Workflow

1. Read the complete diff and
   [network lifecycle inventory](references/network-lifecycle-inventory.md).
   Inspect callers and composition roots, not only changed request helpers.
2. List each affected client, transport, dialer, listener, server, connection,
   pool, ticker, and networking goroutine. Name its constructor, owner, lifetime,
   reuse boundary, and shutdown path.
3. Verify request helpers accept `nethttp.Doer` or an equally narrow injected
   dependency. Require long-running runners to create clients before entering a
   loop, reuse them between iterations, prune stale endpoint clients, and shut
   down owned resources after workers join.
4. Require custom HTTP clients, transports, SOCKS dialers, and servers to be
   constructed through `go/internal/nethttp`. Allow a one-shot owned client only
   when the same scope performs bounded `Shutdown`.
5. Trace every response branch: success, non-200, malformed body, read failure,
   timeout, cancellation, and recovery. Require a bounded read or drain and
   `Body.Close` on every acquired response.
6. Verify listeners stop admission, active work is cancellable and joined, idle
   connections have bounded server-side timeouts, and recovery does not retain
   old pools or workers.
7. Run `make http-lifecycle-check`. Do not accept new or changed
   `nethttp-lifecycle:allow` directives without an explicit declared owner,
   bounded or process lifetime, and specific reason.
8. Run focused tests for every affected package. Include success, non-200,
   malformed response, timeout, cancellation, recovery, reuse, pruning, and
   bounded shutdown cases that apply.
9. When a periodic network flow changes, require resource plateau evidence. Run
   `make resource-plateau` for control-plane behavior and use the nightly gate
   when release evidence is requested. Check fd, TCP state, goroutine, and RSS
   bounds after warm-up; a functional test alone is insufficient.
10. When Go code changes on Windows, run `make test` and `make test-wsl`.
11. Report commands and results, resource owners, exceptions, plateau evidence,
    and residual risks. Treat a missing owner, shutdown join, analyzer result, or
    required plateau result as a blocking finding.

## Review constraints

- Do not propose request timeouts as the fix for an unowned transport or pool.
- Do not accept `CloseIdleConnections` as complete shutdown while requests,
  workers, or stateful dialers can remain active.
- Do not infer lifecycle safety from `defer resp.Body.Close()` alone.
- Do not accept a context-only background owner without an observable completion
  path.
- Keep Desired/Live runtime boundaries and public CLI behavior unchanged unless
  the task explicitly authorizes those changes.
