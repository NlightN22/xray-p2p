import base64
import shutil
import subprocess
from functools import lru_cache
from pathlib import Path
from typing import Iterable

import pytest
import testinfra
from testinfra.backend.base import CommandResult
from testinfra.host import Host

REPO_ROOT = Path(__file__).resolve().parents[2]
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant-win" / "windows10"
DEFAULT_SERVER = "win10-server"
DEFAULT_CLIENT = "win10-client"
BUILD_XP2P_EXE = Path(r"C:\xp2p\build\windows-amd64\xp2p.exe")
PROGRAM_FILES_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
PROGRAM_FILES_BIN_DIR = PROGRAM_FILES_INSTALL_DIR / "bin"
CLIENT_INSTALL_DIR = PROGRAM_FILES_INSTALL_DIR
CLIENT_BIN_DIR = PROGRAM_FILES_BIN_DIR
XP2P_EXE = CLIENT_INSTALL_DIR / "xp2p.exe"
SERVICE_START_TIMEOUT = 60
GUEST_TESTS_ROOT = Path(r"C:\xp2p\tests\guest")


def require_vagrant_environment() -> None:
    if shutil.which("vagrant") is None:
        pytest.skip("Vagrant executable not found on host; guest tests are unavailable.")
    if not VAGRANT_DIR.exists():
        pytest.skip(
            f"Expected Vagrant environment at '{VAGRANT_DIR}' is missing; "
            "run `make vagrant-win10` before invoking host tests."
        )


def ensure_machine_running(machine: str) -> None:
    try:
        state = machine_state(machine)
    except subprocess.CalledProcessError as exc:
        pytest.skip(
            f"Unable to determine state for guest '{machine}' "
            f"(vagrant status exited with code {exc.returncode}). "
            "Run `make vagrant-win10` and retry."
        )
    if state != "running":
        pytest.skip(
            f"Guest '{machine}' is not running (state={state!r}). "
            "Run `make vagrant-win10` and retry."
        )


@lru_cache(maxsize=8)
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


def parse_ssh_config(raw: str) -> dict[str, str]:
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
        if value.startswith('"') and value.endswith('"'):
            value = value[1:-1]
        if key == "identityfile" and key in config:
            continue
        config[key] = value

    required = {"hostname", "user", "port", "identityfile"}
    missing = required.difference(config)
    if missing:
        raise RuntimeError(f"Incomplete ssh-config ({missing}) in output:\n{raw}")
    return config


@lru_cache(maxsize=8)
def _ssh_config(machine: str) -> str:
    return subprocess.check_output(
        ["vagrant", "ssh-config", machine],
        cwd=VAGRANT_DIR,
        text=True,
    )


def get_ssh_host(machine: str) -> Host:
    ensure_machine_running(machine)
    raw = _ssh_config(machine)
    config = parse_ssh_config(raw)
    return testinfra.get_host(
        f"paramiko://{config['user']}@{config['hostname']}:{config['port']}",
        ssh_identity_file=config["identityfile"],
    )


def encode_powershell(script: str) -> str:
    return base64.b64encode(script.encode("utf-16le")).decode("ascii")


def run_powershell(host: Host, script: str) -> CommandResult:
    encoded = encode_powershell(script)
    return host.run(f"powershell -NoProfile -EncodedCommand {encoded}")


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def run_guest_script(host: Host, relative_path: str, **parameters: object) -> CommandResult:
    script_path = GUEST_TESTS_ROOT / relative_path
    ps_path = str(script_path).replace("'", "''")
    args = "".join(f" -{key} {_ps_quote(str(value))}" for key, value in parameters.items())
    command = (
        "powershell -NoProfile -ExecutionPolicy Bypass "
        f"-Command \"& '{ps_path}'{args}\""
    )
    return host.run(command)


def _prepare_program_files_install(machine: str) -> None:
    ensure_host_xp2p_build()
    host = get_ssh_host(machine)
    role = "server" if machine == DEFAULT_SERVER else "client"
    result = run_guest_script(host, "prepare_program_files.ps1", Role=role)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to prepare Program Files xp2p directory:\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def prepare_program_files_install() -> None:
    _prepare_program_files_install(DEFAULT_CLIENT)


def prepare_server_program_files_install() -> None:
    _prepare_program_files_install(DEFAULT_SERVER)


def run_xp2p(host: Host, args: Iterable[str]) -> CommandResult:
    xp2p_ps = str(XP2P_EXE).replace("\\", "\\\\")
    arguments = ", ".join(ps_quote(str(arg)) for arg in args)
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = '{xp2p_ps}'
if (-not (Test-Path $xp2p)) {{
    Write-Output '__XP2P_MISSING__'
    exit 3
}}
$arguments = @({arguments})
& $xp2p @arguments
exit $LASTEXITCODE
"""
    return run_powershell(host, script)


@lru_cache(maxsize=1)
def ensure_host_xp2p_build() -> None:
    build_path = REPO_ROOT / "build" / "windows-amd64" / "xp2p.exe"
    result = subprocess.run(
        ["go", "build", "-o", str(build_path), "./go/cmd/xp2p"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(
            "Failed to build xp2p.exe for host fixtures.\n"
            f"CMD: go build -o {build_path} ./go/cmd/xp2p\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
