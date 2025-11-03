# XP2P Test Authoring Guidelines

This document captures the conventions we now follow when adding or modifying
tests in the `xray-p2p` repository. Keep it close whenever you touch host or
guest suites – the CI and fellow contributors assume these rules.

---

## 1. General Principles
- Prefer **guest-side execution**. Host-driven orchestration should only launch
  a self-contained guest script/test and collect artefacts or return codes.
- **Never introduce new WinRM logic.** We route everything through SSH
  (`testinfra` Paramiko backend) for performance and stability.
- Keep tests **idempotent and clean**. Leave the guest in the same state you
  found it (use fixtures with `yield` and teardown or the shared PowerShell
  helpers).
- Tests must be **hermetic** – no dependence on global state beyond what
  `xp2p_program_files_setup` prepares.

---

## 2. Host-Side Structure (`tests/host`)
1. **Fixtures**
   - Use `server_host` / `client_host` from `tests/host/conftest.py`.
   - Launch long-lived activities through existing helpers:
     - `_env.run_guest_script(...)`
     - `_server_runtime.xp2p_server_run_session(...)`
     - `_client_runtime.xp2p_client_run_session(...)`
   - Need a new helper? Put shared orchestration in `_env.py` or a runtime
     module; keep tests themselves declarative.

2. **PowerShell scripts**
   - Do not inline large PowerShell blobs in Python.
   - Place reusable scripts under `tests/guest/scripts/`.
   - Invoke them with `_env.run_guest_script(host, "scripts/<name>.ps1", ...)`.
   - Parameters must be strings; cast numbers explicitly with `str(...)`.

3. **Assertions & artefacts**
   - Fetch remote files with helper utilities; avoid ad-hoc WinRM or manual
     Base64 wrangling.
   - When capturing logs/configs, store them under `C:\xp2p\artifacts\...`
     inside the guest so the host can read through the synced folder.

---

## 3. Guest-Side Tests (`tests/guest`)
- Keep Go-based smoke tests (e.g. `ping.go`) simple and stateless.
- If you add Python/PowerShell guest tests, ensure they run standalone inside
  the VM (pytest in guest, minimal dependencies).
- Store common PowerShell utilities under `tests/guest/scripts/` and document
  expected parameters at the top of each script.

---

## 4. Performance Expectations
- SSH orchestration should complete quickly; if a test takes >2 minutes, audit
  for redundant provisioning or repeated guest prep.
- `xp2p_program_files_setup` already copies `xp2p.exe` for both guests. Skip
  extra copying unless a test requires a custom binary build.
- Cache results when practical: leverage the `lru_cache` helpers in `_env.py`
  so we do not spam `vagrant status` or `ssh-config`.

---

## 5. Adding New Tests – Checklist
1. Does the scenario truly require host coordination? If not, prefer in-guest
   pytest or Go tests.
2. Are you using `server_host` / `client_host` and the provided helpers?
3. Have you avoided new WinRM usage entirely?
4. Are PowerShell actions stored in a `.ps1` under `tests/guest/scripts/`?
5. Did you clean up temp files/processes in a `finally` block or fixture
   teardown?
6. Can the test run in isolation on a freshly provisioned VM?
7. Did you document non-obvious behaviour or new helpers?

---

## 6. When Unsure
- Check existing host tests (e.g. `test_server.py`, `test_server_users.py`) for
  patterns.
- Ask in code review or update this document if you introduce a new pattern
  that others should reuse.

Keeping to these guidelines ensures the suite stays fast, maintainable, and
friendly to everyone running it locally or in CI. Thanks for sticking to them!
