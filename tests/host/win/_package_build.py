from pathlib import Path

from testinfra.host import Host


def _msi_build_markers() -> tuple[Path, Path, Path]:
    from . import env as _env

    marker_dir = _env.REPO_ROOT / "build" / "msi-build"
    marker_dir.mkdir(parents=True, exist_ok=True)
    token = __import__("uuid").uuid4().hex
    local_path = marker_dir / f"msi-build-{token}.txt"
    guest_start = Path(r"C:\Windows\Temp") / f"xp2p-msi-build-start-{token}.txt"
    guest_done = Path(r"C:\Windows\Temp") / f"xp2p-msi-build-done-{token}.txt"
    return local_path, guest_start, guest_done


def _build_msi_package(
    host: Host,
    *,
    architecture: str,
    cache_dir: Path,
    wix_source: str,
) -> str:
    from . import env as _env

    local_marker, guest_start, guest_done = _msi_build_markers()
    token = local_marker.stem
    if token.startswith("msi-build-"):
        token = token[len("msi-build-") :]
    if local_marker.exists():
        local_marker.unlink(missing_ok=True)

    build_log_out = (
        Path(r"C:\xp2p\build\logs\win") / f"msi-build-{architecture}-{token}-out.log"
    )
    build_log_err = (
        Path(r"C:\xp2p\build\logs\win") / f"msi-build-{architecture}-{token}-err.log"
    )
    build_script = _env.GUEST_TESTS_ROOT / "scripts" / "build_msi_package.ps1"

    def _tail_logs(label: str) -> str:
        tail_script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$out = {_env.ps_quote(str(build_log_out))}
$err = {_env.ps_quote(str(build_log_err))}
if (Test-Path $out) {{
    Write-Output '=== msi build stdout (tail) ==='
    Get-Content -Path $out -Tail 200
}}
if (Test-Path $err) {{
    Write-Output '=== msi build stderr (tail) ==='
    Get-Content -Path $err -Tail 200
}}
"""
        result = _env.run_powershell(host, tail_script, timeout=30, label=label)
        return (result.stdout or "").strip()

    run_script = f"""
$ErrorActionPreference = 'Stop'
$logOutPath = {_env.ps_quote(str(build_log_out))}
$logErrPath = {_env.ps_quote(str(build_log_err))}
$logDir = Split-Path -Parent $logOutPath
if ($logDir -and -not (Test-Path $logDir)) {{
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}}
foreach ($p in @($logOutPath, $logErrPath)) {{
    if (Test-Path $p) {{
        Remove-Item -Path $p -Force -ErrorAction SilentlyContinue
    }}
}}
$startMarker = {_env.ps_quote(str(guest_start))}
$doneMarker = {_env.ps_quote(str(guest_done))}
Remove-Item -Path $startMarker, $doneMarker -Force -ErrorAction SilentlyContinue
$scriptPath = {_env.ps_quote(str(build_script))}
if (-not (Test-Path $scriptPath)) {{
    throw \"MSI build script not found at $scriptPath. Re-mount the synced folder (try 'vagrant reload --provision').\"
}}
$arguments = @(
    '-NoProfile',
    '-ExecutionPolicy', 'Bypass',
    '-File', $scriptPath,
    '-Architecture', {_env.ps_quote(architecture)},
    '-CacheDir', {_env.ps_quote(str(cache_dir))},
    '-WixSource', {_env.ps_quote(wix_source)},
    '-BuildId', {_env.ps_quote(_env._MSI_BUILD_ID or "")},
    '-Marker', {_env.ps_quote(_env.MSI_MARKER)},
    '-StartMarkerPath', $startMarker,
    '-DoneMarkerPath', $doneMarker
)
$proc = Start-Process -FilePath 'powershell' -ArgumentList $arguments -PassThru -RedirectStandardOutput $logOutPath -RedirectStandardError $logErrPath -WindowStyle Hidden
if (-not $proc.WaitForExit(600000)) {{
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    exit 124
}}
exit $proc.ExitCode
"""
    run_result = _env.run_powershell(
        host, run_script, timeout=660, label=f"msi_build_run:{architecture}"
    )
    if run_result.rc != 0:
        tail = _tail_logs(f"msi_build_logs_failed:{architecture}")
        local_marker.write_text(
            f"start={_env._path_exists_raw(host, guest_start)} done={_env._path_exists_raw(host, guest_done)}",
            encoding="ascii",
        )
        if run_result.rc == 124:
            raise RuntimeError(
                f"MSI build timed out after 600s for {architecture}.\n"
                f"Build log tail:\n{tail or '<empty>'}"
            )
        raise RuntimeError(
            f"MSI build failed with exit code {run_result.rc} for {architecture}.\n"
            f"Build log tail:\n{tail or '<empty>'}"
        )

    path: str | None = None
    path_result = _env.run_powershell(
        host,
        f"if (Test-Path {_env.ps_quote(str(guest_done))}) {{ (Get-Content -Raw -Path {_env.ps_quote(str(guest_done))}).Trim() }} else {{ exit 3 }}",
        timeout=30,
        label="msi_build_done_read",
    )
    if path_result.rc == 0:
        path = (path_result.stdout or "").strip()
    if not path:
        out_result = _env.run_powershell(
            host,
            f"if (Test-Path {_env.ps_quote(str(build_log_out))}) {{ Get-Content -Path {_env.ps_quote(str(build_log_out))} -Tail 400 }}",
            timeout=30,
            label="msi_build_log_out_tail",
        )
        path = _env._extract_marker(out_result.stdout or "", _env.MSI_MARKER)
        if path:
            path = path.strip()
    if not path:
        tail = _tail_logs(f"msi_build_logs_missing_marker:{architecture}")
        local_marker.write_text(
            f"start={_env._path_exists_raw(host, guest_start)} done={_env._path_exists_raw(host, guest_done)}",
            encoding="ascii",
        )
        raise RuntimeError(
            f"MSI build completed but artifact path marker is missing for {architecture}.\n"
            f"Build log tail:\n{tail or '<empty>'}"
        )
    if not _env.path_exists(host, path):
        tail = _tail_logs(f"msi_build_logs_missing_artifact:{architecture}")
        raise RuntimeError(
            f"MSI build reported path does not exist: {path}\n"
            f"Build log tail:\n{tail or '<empty>'}"
        )
    return path
