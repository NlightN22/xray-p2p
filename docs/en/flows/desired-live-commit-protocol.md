# Desired-to-Live Commit Protocol

This protocol is the single concurrency boundary for configuration changes.
It applies independently to the `client` and `server` roles and covers CLI
mutations, service compilation, watcher applies, and runtime API applies.

## Module Ownership

- `go/internal/apply.WithRoleLock` owns writer serialization. Every writer that
  can persist Desired inputs, publish Live artifacts, or apply a candidate to a
  running Xray instance must hold the concrete role lock.
- `go/internal/apply.Request` identifies one queued generation. Writing a new
  request replaces an older request for the same role; requests are never
  coalesced by role alone.
- `go/internal/apply.CompleteRequest` is the acknowledgement operation. It
  removes only the exact request ID that was compiled. A newer request remains
  pending.
- `go/internal/apply.SourceDigest` identifies the complete compiler input set:
  the role TOML document and its Desired extension files.
- `go/internal/xraylive` applies and verifies Xray API changes and publishes the
  matching Live directory. Candidate-only applies do not acknowledge unrelated
  queued requests.
- The client and server packages construct domain candidates. They do not own
  request completion or cross-process serialization.
- Client domain snapshots carry the digest of the Desired document they were
  loaded from. The coordinator rejects a stale read-modify-write candidate
  instead of overwriting a newer Desired document.
- Client endpoint selector updates use the same client role lock while reading
  health state, applying a runtime switch, and publishing the selector state
  and journal.

## Invariants

1. There is at most one Desired/Live writer per role.
2. An apply acknowledges only the exact request it read.
3. A newer request cannot be removed by an older apply.
4. Artifacts compiled from a changing Desired input set are not published.
5. A runtime candidate does not consume a service apply request.
6. Live publication replaces the complete role directory atomically and keeps
   the previous directory as LKG.
7. Runtime API success is not complete until matching Live and Desired state is
   persisted. Failure leaves the previously committed state authoritative.
8. A stopped service may commit Desired and queue a new request, but it must not
   publish a fake Live state.
9. Selector state and journal files carry one monotonically increasing revision.
   A reader accepts a pair only when a state-journal-state read observes the
   same revision in all three documents.

## Service-Owned Apply

1. Acquire the role lock.
2. Read the current request and its exact ID.
3. Calculate the Desired source digest.
4. Compile a candidate without changing Live.
5. Recalculate the source digest. If it changed, retain the request and retry
   from a fresh snapshot.
6. Replace the Live role directory and preserve LKG.
7. Acknowledge the exact request ID. If a newer request exists, leave it queued.
8. Release the role lock and start or continue runtime reconciliation from Live.

## Runtime-Capable Command

1. Acquire the role lock before the commit phase.
2. Verify that the candidate still has the current Desired generation, then
   build and validate it. A stale candidate fails and must be reloaded.
3. If Xray is running and the API operation is supported, apply and verify it.
4. Publish matching Live artifacts and persist Desired before reporting success.
5. If the service layer is required or the service is stopped, persist Desired,
   replace the queued request with a fresh request ID, and leave Live unchanged.
6. Release the role lock.

Manual Desired edits do not need to understand the lock. The watcher queues a
fresh request, while the source digest prevents a compiler that observed a
changing input set from publishing it.

## Recovery

- A process exit releases the operating-system role lock automatically.
- An interrupted Live replacement is recovered from the role LKG directory.
- An interrupted apply cannot acknowledge a request unless it reaches the exact
  request completion step.
- A request left after successful publication is safe: it causes an idempotent
  recompile and is acknowledged only by its own generation.
