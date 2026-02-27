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
- **OpenWrt guests are minimal.** Do not change tests to rely on extra packages
  that are not present on OpenWrt (for example `kdig` or `base64`). If a Linux
  test requires additional tooling, add it to the Linux provisioning scripts
  instead of bending OpenWrt tests to match.
- **Build automation stays under `scripts/build/`**. Tests may invoke those
  helpers from guests or hosts, but must not duplicate or relocate build logic
  inside `tests/guest` (e.g., no alternative OpenWrt/DEB/MSI builders under
  the test tree).
- Windows MSI builds from host tests must execute `tests/guest/scripts/build_msi_package.ps1`,
  which wraps the canonical scripts under `scripts/build/`. Keep the MSI build
  pipeline there instead of duplicating commands inside host helpers.
- Prefer shared provisioning scripts under `infra/vagrant/scripts/win`, but
  **OpenSSH is allowed to be local per-VM** when the base image needs special
  handling. Use `infra/vagrant/<vm>/scripts/openssh.ps1` for image-specific
  quirks and keep the rest of the provisioner shared.
- **Never introduce new WinRM logic.** We route everything through SSH
  (`testinfra` Paramiko backend) for performance and stability, even on Windows.
- Keep tests **idempotent and clean**. Leave the guest in the same state you
  found it (use fixtures with `yield`, teardown hooks, or the shared PowerShell
  helpers).
- Tests must be **hermetic** -- no dependence on global state beyond what the
  suite fixtures provision (MSI install on Windows, `.deb` install on Linux, etc.).
- Windows Vagrant VMs run evaluation images; if the license expires Windows License Monitoring Service (`wlms.exe`) shuts the guest down every few hours (Event 1074/User32). Always refresh or re-arm the license before long host test runs, otherwise pytest sessions lose the guest mid-test.

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
- Use the synced `build` tree for temporary files and test-run logs
  (e.g. `C:\xp2p\build\artifacts\...`, `/srv/xray-p2p/build/artifacts/...`);
  reserve `artifacts` for long-lived outputs that must be kept after a run.
- **Do not create application log directories** (for example `C:\ProgramData\xp2p\logs`).
  Tests may write their own logs only under the synced `build/logs/<platform>` tree
  (for example `C:\xp2p\build\logs\win`, `/srv/xray-p2p/build/logs/linux`).
- **OpenWrt reverse tunnel redirects**
  - Add redirect rules only on the ingress side of the tunnel. For client->server
    flows (B->A) this means `xp2p client redirect add --domain ...` on the client
    guest plus a host entry on the server guest so xray resolves the domain. The
    server does **not** need a matching domain rule for the same target. This
    rule applies to both CIDR and domain redirects; configure them only where
    traffic initially enters the tunnel.
  - For server->client flows (A->B) the inverse applies: the server owns the
    redirect (`xp2p server redirect add --domain ...`) and the client provides
    the hosts entry for resolution, while the client does not add an extra
    domain redirect for that same target.
- **Ping assertions**
  - `xp2p ping` invocations must succeed without retries. Do not wrap pings in
    helper loops; invoke once with `check=True` and fail fast when the tunnel is
    misbehaving.

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
- For Linux host suites, build the `.deb` package only once per pytest session
  and reuse it across guests; avoid rebuilding per machine in the same test run.
- Cache results when practical: leverage the `lru_cache` helpers in `_env.py`
  so we do not spam `vagrant status` or `ssh-config`.
- Batch related guest operations into a single `run_powershell` call whenever
  possible (e.g. remove multiple paths or check several files in one script).
  Avoid per-path SSH round-trips for cleanup or state checks.
- Use the PowerShell startup benchmark to spot guest slowness:
  `python scripts/bench/measure_win_powershell_start.py --vagrant-dir infra/vagrant/windows10 --machine win10-a --samples 5`.
  Record the avg/p95 output when comparing images or provisioning changes; the
  target should be ~300ms, otherwise host tests will become very slow.

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

---

## 7. Running Windows Host Tests on Specific Stacks
- Use `--win-stack` to target a specific Vagrant stack.
  Example: `pytest tests/host/win -vv --win-stack win2022`
- Available stacks: `win7`, `win10`, `win2016`, `win2022`.

Following these guidelines keeps the suite fast, maintainable, and friendly to
everyone running it locally or in CI. Thanks for sticking to them!
