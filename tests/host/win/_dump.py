import time
from collections.abc import Iterable
from pathlib import Path

from testinfra.host import Host


def _sanitize_dump_label(label: str) -> str:
    cleaned = []
    last_dash = False
    for char in (label or "").strip().lower():
        if char.isalnum() or char in {"-", "_"}:
            cleaned.append(char)
            last_dash = False
            continue
        if not last_dash:
            cleaned.append("-")
            last_dash = True
    value = "".join(cleaned).strip("-")
    return value or "failure"


def dump_failure_state(
    host: Host,
    *,
    label: str,
    extra_paths: Iterable[Path | str] = (),
) -> Path:
    from . import env as _env

    timestamp = time.strftime("%Y%m%d-%H%M%S")
    backend = getattr(host, "backend", None)
    host_id = getattr(backend, "host", None) or getattr(backend, "hostname", None) or "host"
    safe_label = _sanitize_dump_label(label)
    dump_dir = _env.GUEST_BUILD_ROOT / "logs" / "win"
    dump_path = dump_dir / f"xp2p-failure-{host_id}-{safe_label}-{timestamp}.log"
    ensure_dir = _env.ps_quote(str(dump_dir))
    target = _env.ps_quote(str(dump_path))
    _env.run_powershell(
        host,
        f"""
$ErrorActionPreference = 'SilentlyContinue'
if (-not (Test-Path {ensure_dir})) {{
    New-Item -ItemType Directory -Path {ensure_dir} -Force | Out-Null
}}
if (Test-Path {target}) {{
    Remove-Item -Force {target} -ErrorAction SilentlyContinue
}}
'=== XP2P FAILURE DUMP ({safe_label}) {timestamp} ===' | Out-File -FilePath {target} -Encoding ASCII
""",
        label="dump_failure_state_init",
    )
    _env.run_guest_script(
        host,
        "scripts/dump_net_state.ps1",
        OutputPath=str(dump_path),
        Label=label,
    )
    files = [
        _env.CONFIG_ROOT / "xp2p-client.toml",
        _env.CONFIG_ROOT / "xp2p-server.toml",
        _env.CONFIG_ROOT / "xp2p-client.state.json",
        _env.CONFIG_ROOT / "xp2p-server.state.json",
        _env.CONFIG_ROOT / "state-heartbeat.json",
        _env.CONFIG_ROOT / "state-heartbeat-client.json",
        _env.CONFIG_ROOT / _env.APPLY_DIR_NAME / "apply.request",
        _env.CONFIG_PENDING_ROOT / "xp2p-client.toml",
        _env.CONFIG_PENDING_ROOT / "xp2p-server.toml",
        _env.CONFIG_LIVE_ROOT / "xp2p-client.toml",
        _env.CONFIG_LIVE_ROOT / "xp2p-server.toml",
        _env.CONFIG_LKG_ROOT / "xp2p-client.toml",
        _env.CONFIG_LKG_ROOT / "xp2p-server.toml",
        _env.CONFIG_ROOT / "config-client" / "inbounds.json",
        _env.CONFIG_ROOT / "config-client" / "outbounds.json",
        _env.CONFIG_ROOT / "config-client" / "routing.json",
        _env.CONFIG_ROOT / "config-client" / "logs.json",
        _env.CONFIG_ROOT / "config-server" / "inbounds.json",
        _env.CONFIG_ROOT / "config-server" / "outbounds.json",
        _env.CONFIG_ROOT / "config-server" / "routing.json",
        _env.CONFIG_ROOT / "config-server" / "logs.json",
        _env.CONFIG_PENDING_ROOT / "config-client" / "inbounds.json",
        _env.CONFIG_PENDING_ROOT / "config-client" / "outbounds.json",
        _env.CONFIG_PENDING_ROOT / "config-client" / "routing.json",
        _env.CONFIG_PENDING_ROOT / "config-client" / "logs.json",
        _env.CONFIG_PENDING_ROOT / "config-server" / "inbounds.json",
        _env.CONFIG_PENDING_ROOT / "config-server" / "outbounds.json",
        _env.CONFIG_PENDING_ROOT / "config-server" / "routing.json",
        _env.CONFIG_PENDING_ROOT / "config-server" / "logs.json",
        _env.CONFIG_LIVE_ROOT / "config-client" / "inbounds.json",
        _env.CONFIG_LIVE_ROOT / "config-client" / "outbounds.json",
        _env.CONFIG_LIVE_ROOT / "config-client" / "routing.json",
        _env.CONFIG_LIVE_ROOT / "config-client" / "logs.json",
        _env.CONFIG_LIVE_ROOT / "config-server" / "inbounds.json",
        _env.CONFIG_LIVE_ROOT / "config-server" / "outbounds.json",
        _env.CONFIG_LIVE_ROOT / "config-server" / "routing.json",
        _env.CONFIG_LIVE_ROOT / "config-server" / "logs.json",
        _env.CONFIG_LKG_ROOT / "config-client" / "inbounds.json",
        _env.CONFIG_LKG_ROOT / "config-client" / "outbounds.json",
        _env.CONFIG_LKG_ROOT / "config-client" / "routing.json",
        _env.CONFIG_LKG_ROOT / "config-client" / "logs.json",
        _env.CONFIG_LKG_ROOT / "config-server" / "inbounds.json",
        _env.CONFIG_LKG_ROOT / "config-server" / "outbounds.json",
        _env.CONFIG_LKG_ROOT / "config-server" / "routing.json",
        _env.CONFIG_LKG_ROOT / "config-server" / "logs.json",
    ]
    files.extend(Path(path) for path in extra_paths)
    file_list = ", ".join(_env.ps_quote(str(path)) for path in files)
    roots = ", ".join(
        _env.ps_quote(str(path))
        for path in [
            _env.CONFIG_ROOT,
            _env.CONFIG_ROOT / _env.APPLY_DIR_NAME,
            _env.CONFIG_ROOT / ".apply",
            _env.LOGS_DIR,
            _env.CONFIG_PENDING_ROOT,
            _env.CONFIG_LIVE_ROOT,
            _env.CONFIG_LKG_ROOT,
        ]
    )
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$out = {target}
$lines = New-Object System.Collections.Generic.List[string]
$lines.Add("")
$lines.Add("== config/log roots ==")
$roots = @({roots})
foreach ($root in $roots) {{
    if (-not $root) {{
        continue
    }}
    $lines.Add("-- $root --")
    if (Test-Path $root) {{
        Get-ChildItem -Path $root -Recurse -Force -ErrorAction SilentlyContinue |
            Select-Object FullName,Length,LastWriteTime |
            Format-Table -AutoSize | Out-String | ForEach-Object {{ $lines.Add($_) }}
    }} else {{
        $lines.Add("(missing)")
    }}
}}
$lines.Add("")
$lines.Add("== config/state files ==")
$paths = @({file_list})
foreach ($path in $paths) {{
    if (-not $path) {{
        continue
    }}
    if (Test-Path $path) {{
        $lines.Add("-- $path --")
        Get-Content -Path $path -Raw | ForEach-Object {{ $lines.Add($_) }}
    }}
}}
$lines.Add("")
$lines.Add("== log tails ==")
if (Test-Path {_env.ps_quote(str(_env.LOGS_DIR))}) {{
    $logs = Get-ChildItem -Path {_env.ps_quote(str(_env.LOGS_DIR))} -Filter *.log -Recurse -Force -ErrorAction SilentlyContinue
    foreach ($log in $logs) {{
        $lines.Add("-- $($log.FullName) (tail) --")
        Get-Content -Path $log.FullName -Tail 200 -ErrorAction SilentlyContinue | ForEach-Object {{ $lines.Add($_) }}
    }}
}}
$lines | Out-File -FilePath $out -Append -Encoding ASCII
"""
    _env.run_powershell(host, script, label="dump_failure_state_files")
    print(f"Failure dump written: {dump_path}")
    return dump_path

