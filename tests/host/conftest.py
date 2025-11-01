import base64
import shutil
import subprocess
from pathlib import Path
from typing import Callable, Iterable

import pytest
import testinfra
from testinfra.backend.base import CommandResult

REPO_ROOT = Path(__file__).resolve().parents[2]
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant-win" / "windows10"
DEFAULT_SERVER = "win10-server"
DEFAULT_CLIENT = "win10-client"
XP2P_EXE = Path(r"C:\tools\xp2p\xp2p.exe")
XP2P_EXE_PS = str(XP2P_EXE).replace("\\", "\\\\")
SERVICE_START_TIMEOUT = 60


def pytest_addoption(parser: pytest.Parser) -> None:
    group = parser.getgroup("xp2p", "xp2p guest orchestration options")
    group.addoption(
        "--xp2p-target",
        action="store",
        default="10.0.10.1",
        help="Target address for xp2p guest ping probes.",
    )
    group.addoption(
        "--xp2p-port",
        action="store",
        default="62022",
        help="TCP port for xp2p guest ping probes.",
    )
    group.addoption(
        "--xp2p-attempts",
        action="store",
        type=int,
        default=3,
        help="Number of probe attempts the guest ping should perform.",
    )


@pytest.fixture(scope="session")
def xp2p_options(pytestconfig: pytest.Config) -> dict:
    port_option = pytestconfig.getoption("xp2p_port")
    try:
        port = int(port_option)
    except (TypeError, ValueError):
        pytest.fail(f"Invalid xp2p port value: {port_option!r}")

    return {
        "target": pytestconfig.getoption("xp2p_target"),
        "port": port,
        "attempts": pytestconfig.getoption("xp2p_attempts"),
    }


def _require_vagrant_environment() -> None:
    if shutil.which("vagrant") is None:
        pytest.skip("Vagrant executable not found on host; guest tests are unavailable.")
    if not VAGRANT_DIR.exists():
        pytest.skip(
            f"Expected Vagrant environment at '{VAGRANT_DIR}' is missing; "
            "run `make vagrant-win10` before invoking host tests."
        )


def _machine_state(machine: str) -> str | None:
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


def _parse_winrm_config(raw: str) -> dict[str, str]:
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


def _winrm_hostspec(config: dict[str, str]) -> str:
    return (
        f"winrm://{config['user']}:{config['password']}@"
        f"{config['hostname']}:{config['port']}?no_ssl=true&transport=ntlm"
    )


def _get_testinfra_host(machine: str) -> testinfra.host.Host:
    state = _machine_state(machine)
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
    config = _parse_winrm_config(raw)
    return testinfra.get_host(_winrm_hostspec(config))


@pytest.fixture(scope="session")
def server_host() -> testinfra.host.Host:
    _require_vagrant_environment()
    return _get_testinfra_host(DEFAULT_SERVER)


@pytest.fixture(scope="session")
def client_host() -> testinfra.host.Host:
    _require_vagrant_environment()
    return _get_testinfra_host(DEFAULT_CLIENT)


def _encode_powershell(script: str) -> str:
    return base64.b64encode(script.encode("utf-16le")).decode("ascii")


def _run_powershell(host: testinfra.host.Host, script: str) -> CommandResult:
    encoded = _encode_powershell(script)
    return host.run(f"powershell -NoProfile -EncodedCommand {encoded}")


def _ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


@pytest.fixture
def xp2p_server_service(server_host: testinfra.host.Host, xp2p_options: dict):
    port = xp2p_options["port"]
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = '{XP2P_EXE_PS}'
if (-not (Test-Path $xp2p)) {{
    Write-Output '__XP2P_MISSING__'
    exit 3
}}

$arguments = @('--server-port', '{port}')
$process = Start-Process -FilePath $xp2p -ArgumentList $arguments -WindowStyle Hidden -PassThru
$deadline = (Get-Date).AddSeconds({SERVICE_START_TIMEOUT})

while ((Get-Date) -lt $deadline) {{
    if ($process.HasExited) {{
        Write-Output ('__XP2P_EXIT__' + $process.ExitCode)
        exit $process.ExitCode
    }}
    if (Test-NetConnection -ComputerName '127.0.0.1' -Port {port} -InformationLevel Quiet) {{
        Write-Output ('PID=' + $process.Id)
        exit 0
    }}
    Start-Sleep -Seconds 1
}}

try {{
    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
}} catch {{ }}
Write-Output '__XP2P_TIMEOUT__'
exit 5
"""
    pid_value: int | None = None
    try:
        result = _run_powershell(server_host, script)
        stdout = (result.stdout or "").strip()

        if result.rc != 0:
            if "__XP2P_MISSING__" in stdout:
                pytest.skip(
                    f"xp2p.exe not found on {DEFAULT_SERVER} at {XP2P_EXE}. "
                    "Provision the guest before running host tests."
                )
            pytest.fail(
                "Failed to start xp2p diagnostics service on "
                f"{DEFAULT_SERVER}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

        for line in stdout.splitlines():
            if line.startswith("PID="):
                pid_value = int(line.split("=", 1)[1])
                break
        if pid_value is None:
            pytest.fail(
                "Unexpected xp2p service startup output:\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

        yield {"pid": pid_value, "port": port}
    finally:
        if pid_value is not None:
            stop_script = f"""
$pid = {pid_value}
if ($pid -le 0) {{
    exit 0
}}
$proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
if ($proc) {{
    Stop-Process -Id $pid -Force
}}
exit 0
"""
            _run_powershell(server_host, stop_script)


def _run_xp2p(
    host: testinfra.host.Host,
    args: Iterable[str],
) -> CommandResult:
    arguments = ", ".join(_ps_quote(str(arg)) for arg in args)
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
    return _run_powershell(host, script)


@pytest.fixture
def xp2p_client_runner(
    client_host: testinfra.host.Host,
) -> Callable[..., CommandResult]:
    def _runner(*args: str, check: bool = False):
        result = _run_xp2p(client_host, args)
        stdout = result.stdout or ""
        if "__XP2P_MISSING__" in stdout:
            pytest.skip(
                f"xp2p.exe not found on {DEFAULT_CLIENT} at {XP2P_EXE}. "
                "Provision the guest before running host tests."
            )
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed on "
                f"{DEFAULT_CLIENT} (exit {result.rc}).\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner
