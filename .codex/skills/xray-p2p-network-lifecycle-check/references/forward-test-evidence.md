# Forward-Test Evidence

## Scenario

Review a synthetic network diff containing independent regressions in
subscription sync, continuous ping, HA commit, and SCIM fetch. The reviewer must
use only the skill, its inventory, and the raw diff. The prompt does not disclose
the expected findings.

Input: [forward-test-input.diff](forward-test-input.diff)

SHA-256:
`916386b3a51c24a08c9db435f0a8d3bcf3b4ff154279a350d334fa2ede607206`

## Reproduction Prompt

Run this prompt in a fresh Codex task or subagent from the repository root:

```text
Use $xray-p2p-network-lifecycle-check at
E:\Programming\Go\xray-p2p\.codex\skills\xray-p2p-network-lifecycle-check
to review
E:\Programming\Go\xray-p2p\.codex\skills\xray-p2p-network-lifecycle-check\references\forward-test-input.diff.
Do not edit files. Report whether the change is acceptable, which flows trigger
the skill, blocking findings by flow, required verification commands, plateau
requirements, analyzer exceptions, and residual risks.
```

Do not include this evidence file or the acceptance rubric in the fresh task
context. This prevents the recorded answer from leaking into the evaluation.

## Recorded Run

- Date: 2026-07-27
- Executor: fresh independent Codex subagent with no parent conversation
- Input: the prompt above and `forward-test-input.diff`
- Result: rejected; all four hunks activated the skill
- Repository mutations: none
- Commands executed by the subagent: none

### Findings

| Flow | Blocking findings | Required evidence |
|---|---|---|
| Subscription | Request-scoped `http.Client` and `http.Transport` bypass `go/internal/nethttp`; no injected `Doer`, reuse, pruning, or shutdown; context is unused; response body is neither bounded nor closed. | Focused client lifecycle tests, `make http-lifecycle-check`, and `make resource-plateau` |
| Continuous ping | Ticker is not stopped; probes are admitted without a bound, cancellation, or join; the shown loop can spin without waiting on `ticker.C`. | Focused ping cancellation/overlap tests and plateau evidence for FD, TCP, goroutines, and RSS |
| HA commit | The inline insecure client has no owner or shutdown and cannot be reused across prepare/commit; the response is not closed; context is absent. The shown return type is also incompatible with standard `Post`. | Repeated coordinator lifecycle tests; plateau evidence if the operation becomes periodic |
| SCIM | Request-level client construction bypasses infrastructure and dependency injection; timeout does not establish ownership; context, bounded read, status handling, drain, and body close are absent. | Focused identity sync lifecycle tests and scheduler plateau evidence |

The run required:

```text
make http-lifecycle-check
go test ./go/internal/client -count=1
go test ./go/internal/diagnostics/ping -count=1
go test ./go/internal/ha ./go/internal/server -count=1
go test ./go/internal/identitysync -count=1
make test
make test-wsl
make resource-plateau
```

It additionally required `make resource-plateau-nightly` for release evidence.
It rejected analyzer exclusions for the subscription and SCIM constructors and
noted that inline `Post` plus ticker/goroutine leaks still require manual and
lifecycle-test review even if the HTTP analyzer passes.

### Residual Risks Reported

The subagent reported response leaks, linear transport/FD/TCP/goroutine/RSS
growth, shutdown hangs from requests without context, stale endpoint pools,
lost HA reuse, repeated SCIM clients, and unbounded response memory.

## Acceptance Rubric

An independent rerun passes when it:

1. activates the skill for subscription, ping, HA, and SCIM;
2. rejects the diff;
3. identifies missing ownership and shutdown in all four flows;
4. identifies response handling defects in subscription, HA, and SCIM;
5. identifies unbounded admission and missing join in continuous ping;
6. requires the HTTP analyzer and affected focused tests;
7. requires plateau evidence for periodic flows; and
8. does not propose a timeout as a replacement for ownership.
