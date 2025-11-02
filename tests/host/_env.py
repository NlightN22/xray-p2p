import base64
import shutil
import subprocess
from pathlib import Path
from typing import Iterable

import pytest
import testinfra
from testinfra.host import Host
from testinfra.backend.base import CommandResult

REPO_ROOT = Path(__file__).resolve().parents[2]
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant-win" / "windows10"
DEFAULT_SERVER = "win10-server"
DEFAULT_CLIENT = "win10-client"
BUILD_XP2P_EXE = Path(r"C:\xp2p\build\windows-amd64\xp2p.exe")
CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_BIN_DIR = CLIENT_INSTALL_DIR / "bin"
XP2P_EXE = CLIENT_BIN_DIR / "xp2p.exe"
XP2P_EXE_PS = str(XP2P_EXE).replace("\\", "\\\\")
SERVICE_START_TIMEOUT = 60


def require_vagrant_environment() -> None:
    if shutil.which("vagrant") is None:
        pytest.skip("Vagrant executable not found on host; guest tests are unavailable.")
    if not VAGRANT_DIR.exists():
        pytest.skip(
            f"Expected Vagrant environment at '{VAGRANT_DIR}' is missing; "
            "run `make vagrant-win10` before invoking host tests."
        )


def machine_state(machine: str) -> str | None:
    output = subprocess.check_output(
        ["vagrant", "status", machine, "--machine-readable"],
        cwd=VAGRANT_DIR,
        text=True,
    )
    for line in output.splitlines():
        parts = line.split(",")
        if len(parts) >= 4 and parts[2] == "state":
            return parts[3]
    return None


def parse_winrm_config(raw: str) -> dict[str, str]:
    config: dict[str, str] = {}
    for line in raw.splitlines():
        line = line.strip()
        if not line or line.lower().startswith("host "):
            continue
        pieces = line.split(None, 1)
        if len(pieces) != 2:
            continue
        key = pieces[0].lower()
        value = pieces[1].strip()
        config[key] = value

    required = {"hostname", "user", "password", "port"}
    missing = required.difference(config)
    if missing:
        raise RuntimeError(
            f"Incomplete winrm-config ({missing}) in output:\n{raw}"
        )
    return config


def winrm_hostspec(config: dict[str, str]) -> str:
    return (
        f"winrm://{config['user']}:{config['password']}@"
        f"{config['hostname']}:{config['port']}?no_ssl=true&transport=ntlm"
    )


def get_testinfra_host(machine: str) -> Host:
    state = machine_state(machine)
    if state != "running":
        pytest.skip(
            f"Guest '{machine}' is not running (state={state!r}). "
            "Run `make vagrant-win10` and retry."
        )

    raw = subprocess.check_output(
        ["vagrant", "winrm-config", machine],
        cwd=VAGRANT_DIR,
        text=True,
    )
    config = parse_winrm_config(raw)
    return testinfra.get_host(winrm_hostspec(config))


def encode_powershell(script: str) -> str:
    return base64.b64encode(script.encode("utf-16le")).decode("ascii")


def run_powershell(host: Host, script: str) -> CommandResult:
    encoded = encode_powershell(script)
    return host.run(f"powershell -NoProfile -EncodedCommand {encoded}")


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def run_vagrant_powershell(machine: str, script: str) -> subprocess.CompletedProcess[str]:
    require_vagrant_environment()
    encoded = base64.b64encode(script.encode("utf-16le")).decode("ascii")
    command = [
        "vagrant",
        "winrm",
        machine,
        "--command",
        f"powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -EncodedCommand {encoded}",
    ]
    result: subprocess.CompletedProcess[str] = subprocess.run(
        command,
        cwd=VAGRANT_DIR,
        text=True,
        capture_output=True,
    )
    return result


def prepare_program_files_install() -> None:
    source = str(BUILD_XP2P_EXE).replace("\\", "\\\\")
    root = str(CLIENT_INSTALL_DIR).replace("\\", "\\\\")
    bin_dir = str(CLIENT_BIN_DIR).replace("\\", "\\\\")
    script = f"""
$ErrorActionPreference = 'Stop'
$source = '{source}'
$root = '{root}'
$bin = '{bin_dir}'
if (-not (Test-Path $source)) {{
    throw \"xp2p build binary not found at $source\"
}}
if (Test-Path $root) {{
    try {{
        Remove-Item $root -Recurse -Force -ErrorAction Stop
    }} catch {{
        Remove-Item $root -Recurse -Force -ErrorAction SilentlyContinue
    }}
}}
New-Item -ItemType Directory -Path $bin -Force | Out-Null
Copy-Item $source (Join-Path $bin 'xp2p.exe') -Force
icacls $root /grant 'vagrant:(OI)(CI)M' /t /c | Out-Null
"""
    result = run_vagrant_powershell(DEFAULT_CLIENT, script)
    if result.returncode != 0:
        raise RuntimeError(
            "Failed to prepare Program Files xp2p directory:\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def run_xp2p(host: Host, args: Iterable[str]) -> CommandResult:
    arguments = ", ".join(ps_quote(str(arg)) for arg in args)
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = '{XP2P_EXE_PS}'
if (-not (Test-Path $xp2p)) {{
    Write-Output '__XP2P_MISSING__'
    exit 3
}}
$arguments = @({arguments})
& $xp2p @arguments
exit $LASTEXITCODE
"""
    return run_powershell(host, script)

