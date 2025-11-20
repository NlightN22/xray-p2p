# XP2P Test Authoring Guidelines

This document captures the conventions we follow when adding or modifying
tests in the `xray-p2p` repository. Keep it close whenever you touch host or
guest suites -- the CI and fellow contributors expect these rules.

---

## 1. General Principles
- Prefer **guest-side execution**. Host-driven orchestration should only launch
  a self-contained guest script or test and collect artefacts or return codes.
- **All guest logic lives under `tests/guest/`** (PowerShell/Bash/Go helpers,
  etc.). Host fixtures merely trigger those entrypoints; do not inline ad-hoc
  scripts inside Python.
- **Build automation stays under `scripts/build/`**. Tests may invoke those
  helpers from guests or hosts, but must not duplicate or relocate build logic
  inside `tests/guest` (e.g., no alternative OpenWrt/DEB/MSI builders under
  the test tree).
- Windows MSI builds from host tests must execute `tests/guest/scripts/build_msi_package.ps1`,
  which wraps the canonical scripts under `scripts/build/`. Keep the MSI build
  pipeline there instead of duplicating commands inside host helpers.
- **Never introduce new WinRM logic.** We route everything through SSH
  (`testinfra` Paramiko backend) for performance and stability, even on Windows.
- Keep tests **idempotent and clean**. Leave the guest in the same state you
  found it (use fixtures with `yield`, teardown hooks, or the shared PowerShell
  helpers).
- Tests must be **hermetic** -- no dependence on global state beyond what the
  suite fixtures provision (MSI install on Windows, `.deb` install on Linux, etc.).

---

## 2. Host-Side Structure (`tests/host`)
1. **Fixtures**
   - Use the fixtures exposed by the platform package (e.g.
     `tests.host.win.conftest`, `tests.host.linux.conftest`) to obtain hosts or
     xp2p runners.
   - Launch long-lived activities through existing helpers:
     - `_env.run_guest_script(...)`
     - `_server_runtime.xp2p_server_run_session(...)`
     - `_client_runtime.xp2p_client_run_session(...)`
   - Need a new helper? Put shared orchestration in the relevant `env.py` or a runtime
     module; keep tests themselves declarative.

2. **Guest scripts**
   - Do not inline large PowerShell/Bash blobs in Python.
   - Place reusable scripts under `tests/guest/scripts/` (match the platform,
     e.g. `.ps1` for Windows, `.sh`/`.py` for Linux guests).
   - Invoke them with `_env.run_guest_script(host, "scripts/<name>.<ext>", ...)`.
   - Parameters must be strings; cast numbers explicitly with `str(...)`.

3. **Assertions and artefacts**
   - Fetch remote files with helper utilities; avoid ad-hoc transport hacks.
   - When capturing logs/configs, store them under the synced root so the host
     can read them (e.g. `C:\xp2p\artifacts\...` on Windows,
     `/srv/xray-p2p/artifacts/...` on Linux).

---

## 3. Guest-Side Tests (`tests/guest`)
- Keep Go-based smoke tests (for example `ping.go`) simple and stateless.
- If you add Python or PowerShell guest tests, ensure they run standalone inside
  the VM (pytest in guest, minimal dependencies).
- Store common PowerShell utilities under `tests/guest/scripts/` and document
  expected parameters at the top of each script.
- Build/install automation that multiple suites reuse belongs in `scripts/build/`
  (for example `build_deb_xp2p.sh`). Host tests invoke those scripts instead of
  duplicating build logic inline.

---

## 4. Performance Expectations
- SSH orchestration should complete quickly; if a test takes more than
  two minutes, audit for redundant provisioning or repeated guest prep.
- Platform fixtures already install xp2p (MSI on Windows, `.deb` on Debian). Skip
  extra copying unless a test requires a custom build; in that case, use the
  helper scripts from `scripts/build/` and stage artefacts via the synced folder.
- Cache results when practical: leverage the `lru_cache` helpers in `_env.py`
  so we do not spam `vagrant status` or `ssh-config`.

---

## 5. Adding New Tests -- Checklist
1. Does the scenario truly require host coordination? If not, prefer in-guest
   pytest or Go tests.
2. Are you using platform fixtures (`server_host`, `client_host`, Linux machine
   factories, etc.) and the provided helpers?
3. Have you avoided new WinRM usage entirely?
4. Are guest actions stored in `tests/guest/scripts/` (with the right extension)?
5. Did you clean up temporary files or processes in a `finally` block or fixture
   teardown?
6. Can the test run in isolation on a freshly provisioned VM?
7. Did you document non-obvious behaviour or new helpers?

---

## 6. When Unsure
- Check existing host tests (for example `test_server_install.py`,
  `test_server_users.py`) for patterns.
- Ask in code review or update this document if you introduce a new pattern
  that others should reuse.

Following these guidelines keeps the suite fast, maintainable, and friendly to
everyone running it locally or in CI. Thanks for sticking to them!
