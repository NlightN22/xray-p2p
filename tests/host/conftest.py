from typing import Callable

import pytest
from testinfra.backend.base import CommandResult
from testinfra.host import Host

from . import _client_runtime, _env


def pytest_addoption(parser: pytest.Parser) -> None:
    group = parser.getgroup("xp2p", "xp2p guest orchestration options")
    group.addoption(
        "--xp2p-target",
        action="store",
        default="10.0.10.10",
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


@pytest.fixture(scope="session")
def server_host() -> Host:
    _env.require_vagrant_environment()
    return _env.get_testinfra_host(_env.DEFAULT_SERVER)


@pytest.fixture(scope="session")
def client_host() -> Host:
    _env.require_vagrant_environment()
    return _env.get_testinfra_host(_env.DEFAULT_CLIENT)


@pytest.fixture
def xp2p_server_service(server_host: Host, xp2p_options: dict):
    port = xp2p_options["port"]
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = '{_env.XP2P_EXE_PS}'
if (-not (Test-Path $xp2p)) {{
    Write-Output '__XP2P_MISSING__'
    exit 3
}}

$existing = Get-Process -Name xp2p -ErrorAction SilentlyContinue | Where-Object {{ $_.Path -eq $xp2p }}
if ($existing) {{
    foreach ($item in $existing) {{
        try {{
            Stop-Process -Id $item.Id -Force -ErrorAction SilentlyContinue
        }} catch {{ }}
    }}
    Start-Sleep -Seconds 1
    $remaining = Get-Process -Name xp2p -ErrorAction SilentlyContinue | Where-Object {{ $_.Path -eq $xp2p }}
    if ($remaining) {{
        Write-Output '__XP2P_ALREADY_RUNNING__'
        exit 7
    }}
}}

$commandLine = "`"$xp2p`" --server-port {port}"
$workingDir = Split-Path $xp2p
$createResult = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{{ CommandLine = $commandLine; CurrentDirectory = $workingDir }}
if ($createResult.ReturnValue -ne 0 -or -not $createResult.ProcessId) {{
    Write-Output ('__XP2P_CREATE_FAIL__' + $createResult.ReturnValue)
    exit 4
}}
$processId = [int]$createResult.ProcessId
$deadline = (Get-Date).AddSeconds({_env.SERVICE_START_TIMEOUT})

while ((Get-Date) -lt $deadline) {{
    $proc = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if (-not $proc) {{
        Write-Output '__XP2P_EXIT__'
        exit 6
    }}
    if (Test-NetConnection -ComputerName '127.0.0.1' -Port {port} -InformationLevel Quiet) {{
        Write-Output ('PID=' + $processId)
        exit 0
    }}
    Start-Sleep -Seconds 1
}}

$proc = Get-Process -Id $processId -ErrorAction SilentlyContinue
if ($proc) {{
    try {{
        Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
    }} catch {{ }}
}}
Write-Output '__XP2P_TIMEOUT__'
exit 5
"""
    pid_value: int | None = None
    try:
        result = _env.run_powershell(server_host, script)
        stdout = (result.stdout or "").strip()

        if result.rc != 0:
            if "__XP2P_MISSING__" in stdout:
                pytest.skip(
                    f"xp2p.exe not found on {_env.DEFAULT_SERVER} at {_env.XP2P_EXE}. "
                    "Provision the guest before running host tests."
                )
            if "__XP2P_CREATE_FAIL__" in stdout:
                pytest.fail(
                    "Failed to spawn xp2p diagnostics service via Win32_Process.\n"
                    f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
                )
            if "__XP2P_ALREADY_RUNNING__" in stdout:
                pytest.skip(
                    "xp2p diagnostics service is already running on the server; "
                    "stop manual instances before executing host tests."
                )
            pytest.fail(
                "Failed to start xp2p diagnostics service on "
                f"{_env.DEFAULT_SERVER}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

        for line in stdout.splitlines():
            if line == "__XP2P_EXIT__":
                pytest.fail(
                    "xp2p diagnostics service exited before the port was reachable.\n"
                    f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
                )
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
$pidValue = {pid_value}
if ($pidValue -le 0) {{
    exit 0
}}
$proc = Get-Process -Id $pidValue -ErrorAction SilentlyContinue
if ($proc) {{
    Stop-Process -Id $pidValue -Force
}}
exit 0
"""
            _env.run_powershell(server_host, stop_script)


@pytest.fixture
def xp2p_client_run_factory(client_host: Host):
    def _factory():
        return _client_runtime.xp2p_client_run_session(client_host)

    return _factory


@pytest.fixture
def xp2p_client_runner(
    client_host: Host,
) -> Callable[..., CommandResult]:
    def _runner(*args: str, check: bool = False):
        result = _env.run_xp2p(client_host, args)
        stdout = result.stdout or ""
        if "__XP2P_MISSING__" in stdout:
            pytest.skip(
                f"xp2p.exe not found on {_env.DEFAULT_CLIENT} at {_env.XP2P_EXE}. "
                "Provision the guest before running host tests."
            )
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed on "
                f"{_env.DEFAULT_CLIENT} (exit {result.rc}).\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner
