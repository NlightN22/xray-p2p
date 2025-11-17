from typing import Callable

import pytest
from testinfra.backend.base import CommandResult
from testinfra.host import Host

from . import _client_runtime, _server_runtime, env as win_env


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
def xp2p_msi_path(server_host: Host) -> str:
    return win_env.ensure_msi_package(server_host)


@pytest.fixture(scope="session", autouse=True)
def xp2p_program_files_setup(server_host: Host, client_host: Host, xp2p_msi_path: str):
    win_env.install_xp2p_from_msi(server_host, xp2p_msi_path)
    win_env.install_xp2p_from_msi(client_host, xp2p_msi_path)
    yield


@pytest.fixture(scope="session")
def server_host() -> Host:
    win_env.require_vagrant_environment()
    return win_env.get_ssh_host(win_env.DEFAULT_SERVER)


@pytest.fixture(scope="session")
def client_host() -> Host:
    win_env.require_vagrant_environment()
    return win_env.get_ssh_host(win_env.DEFAULT_CLIENT)


@pytest.fixture
def xp2p_server_service(server_host: Host, xp2p_options: dict):
    port = xp2p_options["port"]
    pid_value: int | None = None
    try:
        result = win_env.run_guest_script(
            server_host,
            "scripts/start_xp2p_service.ps1",
            Xp2pPath=str(win_env.XP2P_EXE),
            Port=port,
            TimeoutSeconds=win_env.SERVICE_START_TIMEOUT,
        )
        stdout = (result.stdout or "").strip()

        if result.rc != 0:
            if "__XP2P_MISSING__" in stdout:
                pytest.skip(
                    f"xp2p.exe not found on {win_env.DEFAULT_SERVER} at {win_env.XP2P_EXE}. "
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
                f"{win_env.DEFAULT_SERVER}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
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
            win_env.run_powershell(server_host, stop_script)


@pytest.fixture
def xp2p_client_run_factory(client_host: Host):
    def _factory(install_dir: str, config_dir: str, log_relative: str):
        return _client_runtime.xp2p_client_run_session(client_host, install_dir, config_dir, log_relative)

    return _factory


@pytest.fixture
def xp2p_server_run_factory(server_host: Host):
    def _factory(install_dir: str, config_dir: str, log_relative: str):
        return _server_runtime.xp2p_server_run_session(server_host, install_dir, config_dir, log_relative)

    return _factory


@pytest.fixture
def xp2p_client_runner(
    client_host: Host,
) -> Callable[..., CommandResult]:
    def _runner(*args: str, check: bool = False):
        cmd = list(args)
        if len(cmd) >= 2 and cmd[0] in {"client", "server"} and cmd[1] == "remove":
            if "--quiet" not in cmd:
                cmd.append("--quiet")
        result = win_env.run_xp2p(client_host, cmd)
        stdout = result.stdout or ""
        if "__XP2P_MISSING__" in stdout:
            pytest.skip(
                f"xp2p.exe not found on {win_env.DEFAULT_CLIENT} at {win_env.XP2P_EXE}. "
                "Provision the guest before running host tests."
            )
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed on "
                f"{win_env.DEFAULT_CLIENT} (exit {result.rc}).\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner


@pytest.fixture
def xp2p_server_runner(
    server_host: Host,
) -> Callable[..., CommandResult]:
    def _runner(*args: str, check: bool = False):
        cmd = list(args)
        if len(cmd) >= 2 and cmd[0] in {"client", "server"} and cmd[1] == "remove":
            if "--quiet" not in cmd:
                cmd.append("--quiet")
        result = win_env.run_xp2p(server_host, cmd)
        stdout = result.stdout or ""
        if "__XP2P_MISSING__" in stdout:
            pytest.skip(
                f"xp2p.exe not found on {win_env.DEFAULT_SERVER} at {win_env.XP2P_EXE}. "
                "Provision the guest before running host tests."
            )
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed on "
                f"{win_env.DEFAULT_SERVER} (exit {result.rc}).\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner
