import time
import uuid
from contextlib import contextmanager
from typing import Callable

import pytest
from testinfra.backend.base import CommandResult
from testinfra.host import Host

from . import _client_runtime, _server_runtime, env as win_env

_LAST_TEST_END: float | None = None


@contextmanager
def _timed(label: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: {label}: {elapsed:.2f}s")


def pytest_addoption(parser: pytest.Parser) -> None:
    group = parser.getgroup("xp2p", "xp2p guest orchestration options")
    group.addoption(
        "--win-stack",
        action="store",
        default="win10",
        help="Windows Vagrant stack to target (win7, win10, win2016, win2022).",
    )
    group.addoption(
        "--xp2p-target",
        action="store",
        default=None,
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
    group.addoption(
        "--run-msi-build-tests",
        action="store_true",
        default=False,
        help="Run MSI build validation tests (default: skip).",
    )


def pytest_collection_modifyitems(config: pytest.Config, items: list[pytest.Item]) -> None:
    if config.getoption("run_msi_build_tests"):
        return
    skip_marker = pytest.mark.skip(reason="MSI build tests are skipped by default.")
    for item in items:
        nodeid = item.nodeid
        if "tests/host/win/test_installer_msi.py::test_windows_installer_builds_msi" in nodeid:
            item.add_marker(skip_marker)
        if "tests/host/win/test_installer_msi.py::test_windows_installer_builds_msi_x86" in nodeid:
            item.add_marker(skip_marker)


@pytest.fixture(autouse=True)
def _timed_test(request: pytest.FixtureRequest):
    global _LAST_TEST_END
    start = time.perf_counter()
    if _LAST_TEST_END is not None:
        gap = start - _LAST_TEST_END
        if gap >= 0.5:
            print(f"TIMING: gap before {request.node.nodeid}: {gap:.2f}s")
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: test {request.node.nodeid} total: {elapsed:.2f}s")
        _LAST_TEST_END = time.perf_counter()


@pytest.fixture(scope="session")
def xp2p_options(pytestconfig: pytest.Config) -> dict:
    target = pytestconfig.getoption("xp2p_target")
    if not target:
        target = win_env.DEFAULT_TARGET

    port_option = pytestconfig.getoption("xp2p_port")
    try:
        port = int(port_option)
    except (TypeError, ValueError):
        pytest.fail(f"Invalid xp2p port value: {port_option!r}")

    return {
        "target": target,
        "port": port,
        "attempts": pytestconfig.getoption("xp2p_attempts"),
    }


@pytest.fixture(scope="session", autouse=True)
def _configure_win_stack(pytestconfig: pytest.Config) -> None:
    name = pytestconfig.getoption("win_stack")
    try:
        win_env.set_win_stack(name)
    except ValueError as exc:
        pytest.fail(str(exc))


@pytest.fixture(scope="session")
def xp2p_build_id() -> str:
    return uuid.uuid4().hex


@pytest.fixture(scope="session", autouse=True)
def _configure_msi_build_id(xp2p_build_id: str) -> None:
    win_env.set_msi_build_id(xp2p_build_id)


@pytest.fixture(scope="session")
def xp2p_msi_path() -> str:
    win_env.require_vagrant_environment()
    with _timed("get_ssh_host (msi build)"):
        server_host = win_env.get_ssh_host(win_env.DEFAULT_SERVER)
    with _timed("ensure_msi_package"):
        return win_env.ensure_msi_package(server_host)


@pytest.fixture(scope="session", autouse=True)
def xp2p_program_files_setup():
    win_env.require_vagrant_environment()
    with _timed("get_ssh_host (server setup)"):
        server_host = win_env.get_ssh_host(win_env.DEFAULT_SERVER)
    with _timed("get_ssh_host (client setup)"):
        client_host = win_env.get_ssh_host(win_env.DEFAULT_CLIENT)
    with _timed("ensure_admin_token (server)"):
        win_env.ensure_admin_token(server_host)
    with _timed("ensure_admin_token (client)"):
        win_env.ensure_admin_token(client_host)
    server_backend = getattr(server_host, "backend", None)
    client_backend = getattr(client_host, "backend", None)
    if hasattr(server_backend, "_reset_client"):
        server_backend._reset_client()
    if hasattr(client_backend, "_reset_client"):
        client_backend._reset_client()
    with _timed("ensure_project_synced (server)"):
        win_env.ensure_project_synced(server_host, machine=win_env.DEFAULT_SERVER)
    with _timed("ensure_project_synced (client)"):
        win_env.ensure_project_synced(client_host, machine=win_env.DEFAULT_CLIENT)
    with _timed("ensure_program_files_install (server)"):
        win_env.ensure_program_files_install(server_host, force_reinstall=True)
    detected_server = win_env.find_xp2p_exe(server_host, hint_path=win_env.XP2P_EXE)
    print(f"INFO: server xp2p.exe detected at {detected_server}")
    if detected_server is None:
        pytest.fail(f"xp2p.exe not detected on {win_env.DEFAULT_SERVER} after install")
    with _timed("ensure_program_files_install (client)"):
        win_env.ensure_program_files_install(client_host, force_reinstall=True)
    detected_client = win_env.find_xp2p_exe(client_host, hint_path=win_env.XP2P_EXE)
    print(f"INFO: client xp2p.exe detected at {detected_client}")
    if detected_client is None:
        pytest.fail(f"xp2p.exe not detected on {win_env.DEFAULT_CLIENT} after install")
    yield
    with _timed("ensure_msi_package (teardown)"):
        msi_path = win_env.ensure_msi_package(server_host)
    with _timed("uninstall_xp2p_from_msi (server)"):
        win_env.uninstall_xp2p_from_msi(server_host, msi_path)
    with _timed("uninstall_xp2p_from_msi (client)"):
        win_env.uninstall_xp2p_from_msi(client_host, msi_path)


@pytest.fixture
def server_host() -> Host:
    win_env.require_vagrant_environment()
    with _timed("get_ssh_host (server)"):
        return win_env.get_ssh_host(win_env.DEFAULT_SERVER)


@pytest.fixture
def client_host() -> Host:
    win_env.require_vagrant_environment()
    with _timed("get_ssh_host (client)"):
        return win_env.get_ssh_host(win_env.DEFAULT_CLIENT)


@pytest.fixture(scope="session")
def server_host_ipv4(server_host: Host) -> str:
    with _timed("detect server IPv4 (session)"):
        return win_env.get_host_ipv4(server_host)


@pytest.fixture(scope="session")
def client_host_ipv4(client_host: Host) -> str:
    with _timed("detect client IPv4 (session)"):
        return win_env.get_host_ipv4(client_host)


@pytest.fixture
def xp2p_server_service(server_host: Host, xp2p_options: dict):
    win_env.ensure_program_files_install(server_host)
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
    win_env.ensure_program_files_install(client_host)

    def _factory(install_dir: str, config_dir: str, log_relative: str):
        return _client_runtime.xp2p_client_run_session(client_host, install_dir, config_dir, log_relative)

    return _factory


@pytest.fixture
def xp2p_server_run_factory(server_host: Host):
    win_env.ensure_program_files_install(server_host)

    def _factory(install_dir: str, config_dir: str, log_relative: str):
        return _server_runtime.xp2p_server_run_session(server_host, install_dir, config_dir, log_relative)

    return _factory


@pytest.fixture
def xp2p_client_runner(
    client_host: Host,
) -> Callable[..., CommandResult]:
    win_env.ensure_program_files_install(client_host)

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
    win_env.ensure_program_files_install(server_host)

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
