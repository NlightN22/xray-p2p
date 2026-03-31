from __future__ import annotations

from pathlib import PurePosixPath
import time

import pytest

from tests.host import config_files
from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env
try:
    import tomllib
except ImportError:  # pragma: no cover - fallback for older runtimes.
    import tomli as tomllib

CLIENT_SERVICE_LOG = helpers.LOG_ROOT / "client" / "service.log"
CLIENT_XRAY_LOG = helpers.LOG_ROOT / "client" / "xray-service.log"
SERVER_SERVICE_LOG = helpers.LOG_ROOT / "server" / "service.log"
SERVER_XRAY_LOG = helpers.LOG_ROOT / "server" / "xray-service.log"
CLIENT_CONFIG = helpers.CLIENT_CONFIG_FILE
SERVER_CONFIG = helpers.SERVER_CONFIG_FILE
APPLY_REQUEST = helpers.CONFIG_ROOT / ".apply" / "apply.request"
CLIENT_DIAG_PORT = "62023"
SERVER_DIAG_PORT = "62022"

SERVICE_TIMEOUT = 45.0
POLL_INTERVAL = 1.5


def _wait_for_service_state(runner, role: str, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    last_result = None
    while time.time() < deadline:
        result = runner(role, "service", "status")
        active = result.rc == 0
        if active == expected_active:
            return
        last_result = result
        time.sleep(POLL_INTERVAL)

    state = "active" if expected_active else "inactive"
    stdout = (last_result.stdout or "") if last_result else ""
    stderr = (last_result.stderr or "") if last_result else ""
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state. "
        f"Last rc: {getattr(last_result, 'rc', 'n/a')}\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def _install_client_endpoint(runner, host_addr: str, user: str, password: str, *, sni: str | None = None) -> None:
    args = [
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--host",
        host_addr,
        "--user",
        user,
        "--password",
        password,
        "--force",
    ]
    if sni:
        args.extend(["--sni", sni])
    runner(*args, check=True)


def _install_server_instance(runner, host, host_name: str = "svc-server.example.com", port: str = "62120") -> None:
    runner(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--port",
        port,
        "--host",
        host_name,
        "--force",
        check=True,
    )
    host.run("sudo -n systemctl daemon-reload >/dev/null 2>&1 || true")


def _start_service(role: str, runner, host, diag_port: str | None) -> None:
    host.run("sudo -n systemctl daemon-reload >/dev/null 2>&1 || true")
    runner(role, "service", "stop")
    runner(role, "service", "start", check=True)
    _wait_for_service_state(runner, role, expected_active=True)
    if diag_port:
        _assert_diag_listener(host, diag_port)


def _stop_service(role: str, runner) -> None:
    runner(role, "service", "stop")
    _wait_for_service_state(runner, role, expected_active=False)


def _unit_missing(result) -> bool:
    text = ((result.stdout or "") + (result.stderr or "")).lower()
    return "could not be found" in text or "loaded: not-found" in text


def _assert_service_inactive(host, service_name: str) -> None:
    result = host.run(f"sudo -n systemctl is-active {service_name}")
    assert result.rc != 0, f"{service_name} should not be active"


def _assert_diag_listener(host, port: str) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    while time.time() < deadline:
        result = host.run(f"sudo -n ss -lntp | grep :{port}")
        if result.rc == 0:
            return
        time.sleep(POLL_INTERVAL)
    pytest.fail(f"Expected diagnostics listener on port {port} after {SERVICE_TIMEOUT} seconds.")


def _wait_for_log_entry(host, path, substring: str, timeout: float = 60.0) -> None:
    deadline = time.time() + timeout
    lowered = substring.lower()
    last_content = ""
    while time.time() < deadline:
        if helpers.path_exists(host, path):
            content = helpers.read_text(host, path)
            last_content = content
            if lowered in content.lower():
                return
        time.sleep(POLL_INTERVAL)
    raise AssertionError(f"Log {path} did not contain {substring!r}. Last content:\n{last_content}")


def _wait_for_apply_request_clear(host, timeout: float = 60.0) -> None:
    deadline = time.time() + timeout
    request_path = helpers.CONFIG_ROOT / ".apply" / "apply.request"
    while time.time() < deadline:
        if not linux_env.path_exists(host, request_path):
            return
        time.sleep(POLL_INTERVAL)
    raise AssertionError(f"apply.request did not clear after {timeout} seconds.")


def _write_apply_request(host, role: str) -> None:
    helpers.write_apply_request(host, role)


def _current_mode(host, role: str) -> str:
    state = helpers.read_client_config(host) if role == "client" else helpers.read_server_config(host)
    tun_enabled = state.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        raise AssertionError(f"Expected tun_enabled boolean in {role} config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def _current_live_mode(host, role: str) -> str:
    config_path = CLIENT_CONFIG if role == "client" else SERVER_CONFIG
    content = linux_env.read_text(host, config_path)
    try:
        tree = tomllib.loads(content)
    except tomllib.TOMLDecodeError as exc:
        raise AssertionError(f"Failed to parse TOML from {config_path}: {exc}\nContent:\n{content}") from exc
    state = tree.get(role) or {}
    tun_enabled = state.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        raise AssertionError(f"Expected tun_enabled boolean in {role} live config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def _set_mode(runner, role: str, config_dir: str, mode: str) -> None:
    runner(
        role,
        "mode",
        mode,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        config_dir,
        check=True,
    )


def _toggle_mode(host, runner, role: str, config_dir: str) -> str:
    previous = _current_mode(host, role)
    target = "proxy" if previous == "tun" else "tun"
    _set_mode(runner, role, config_dir, target)
    return previous


@pytest.mark.host
@pytest.mark.linux
def test_client_service_cli_controls_systemd(client_host, xp2p_client_runner):
    try:
        _install_client_endpoint(
            xp2p_client_runner,
            "10.55.10.50",
            "svc-client@example.com",
            "svc-client-secret",
        )

        _start_service("client", xp2p_client_runner, client_host, CLIENT_DIAG_PORT)
        assert helpers.path_exists(client_host, CLIENT_SERVICE_LOG), "client service log was not created"

        _stop_service("client", xp2p_client_runner)
        assert helpers.path_exists(client_host, CLIENT_SERVICE_LOG), "client service log missing after stop"
    finally:
        _stop_service("client", xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_service_cli_controls_systemd(server_host, xp2p_server_runner):
    try:
        _install_server_instance(xp2p_server_runner, server_host, host_name="svc-server.example.com", port="62120")

        _start_service("server", xp2p_server_runner, server_host, SERVER_DIAG_PORT)
        assert helpers.path_exists(server_host, SERVER_SERVICE_LOG), "server service log was not created"

        _stop_service("server", xp2p_server_runner)
        assert helpers.path_exists(server_host, SERVER_SERVICE_LOG), "server service log missing after stop"
    finally:
        _stop_service("server", xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize("role", ["client", "server"])
def test_service_restarts_when_config_changes(
    role,
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    if role == "client":
        host = client_host
        runner = xp2p_client_runner
        service_log = CLIENT_SERVICE_LOG
        xray_log = CLIENT_XRAY_LOG
        diag_port = CLIENT_DIAG_PORT
        config_dir = helpers.CLIENT_CONFIG_DIR_NAME
        base_host = "10.55.10.60"
        original_mode: str | None = None
        install_fn = lambda: _install_client_endpoint(
            xp2p_client_runner,
            base_host,
            "svc-change@example.com",
            "svc-change-secret",
        )

        def change_fn():
            nonlocal original_mode
            original_mode = _toggle_mode(host, runner, role, config_dir)

        def revert_fn():
            if original_mode:
                _set_mode(runner, role, config_dir, original_mode)
    else:
        host = server_host
        runner = xp2p_server_runner
        service_log = SERVER_SERVICE_LOG
        xray_log = SERVER_XRAY_LOG
        diag_port = SERVER_DIAG_PORT
        config_dir = helpers.SERVER_CONFIG_DIR_NAME
        original_mode: str | None = None
        install_fn = lambda: _install_server_instance(
            xp2p_server_runner,
            host,
            host_name="svc-server.example.com",
            port="62125",
        )

        def change_fn():
            nonlocal original_mode
            original_mode = _toggle_mode(host, runner, role, config_dir)

        def revert_fn():
            if original_mode:
                _set_mode(runner, role, config_dir, original_mode)

    try:
        install_fn()
        _start_service(role, runner, host, diag_port)

        change_fn()
        try:
            _wait_for_apply_request_clear(host)
            _wait_for_service_state(runner, role, expected_active=True)
            expected_mode = "proxy" if original_mode == "tun" else "tun"
            assert _current_live_mode(host, role) == expected_mode
        finally:
            revert_fn()
    finally:
        _stop_service(role, runner)


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize("role", ["client", "server"])
def test_service_stops_after_invalid_config(
    role,
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    max_attempts = 5
    if role == "client":
        host = client_host
        runner = xp2p_client_runner
        service_log = CLIENT_SERVICE_LOG
        xray_log = CLIENT_XRAY_LOG
        config_path = CLIENT_CONFIG
        invalid_config = "[client]\nendpoints = \"invalid\"\n"
        diag_port = CLIENT_DIAG_PORT
        install_fn = lambda: _install_client_endpoint(
            xp2p_client_runner,
            "10.55.10.70",
            "svc-fail@example.com",
            "svc-fail-secret",
        )
    else:
        host = server_host
        runner = xp2p_server_runner
        service_log = SERVER_SERVICE_LOG
        xray_log = SERVER_XRAY_LOG
        config_path = SERVER_CONFIG
        invalid_config = "[server]\nserver_redirects = \"invalid\"\n"
        diag_port = SERVER_DIAG_PORT
        install_fn = lambda: _install_server_instance(
            xp2p_server_runner,
            host,
            host_name="svc-server.example.com",
            port="62130",
        )

    original_config = None
    try:
        install_fn()
        original_config = helpers.read_text(host, config_path)
        _start_service(role, runner, host, diag_port)

        helpers.write_text(host, config_path, invalid_config)
        _write_apply_request(host, role)
        _wait_for_log_entry(host, service_log, "service configuration change detected")
        _wait_for_log_entry(host, service_log, "exceeded restart limit")
        _wait_for_service_state(runner, role, expected_active=False)

        log_content = helpers.read_text(host, service_log)
        attempts = log_content.lower().count("attempt:")
        assert attempts >= max_attempts, f"expected at least {max_attempts} restart attempts"
        assert helpers.path_exists(host, xray_log), f"{role} xray log missing"
        if role == "client":
            assert helpers.read_text(host, xray_log).strip(), f"{role} xray log is empty"
    finally:
        if original_config is not None:
            helpers.write_text(host, config_path, original_config)
        runner(role, "service", "stop")


@pytest.mark.host
@pytest.mark.linux
def test_package_uninstall_removes_services(server_host):
    install_root = helpers.INSTALL_ROOT
    log_root = helpers.LOG_ROOT
    bin_path = PurePosixPath("/usr/bin/xp2p")
    service_paths = [
        PurePosixPath("/lib/systemd/system/xp2p-client.service"),
        PurePosixPath("/lib/systemd/system/xp2p-server.service"),
    ]

    def _assert_missing(path: PurePosixPath) -> None:
        assert not helpers.path_exists(server_host, path), f"{path} should be removed"

    def _run_logged(label: str, command: str, *, timeout: int | None = None):
        start = time.perf_counter()
        result = server_host.run(command, timeout=timeout) if timeout else server_host.run(command)
        elapsed = time.perf_counter() - start
        print(f"DEBUG: {label} rc={result.rc} elapsed={elapsed:.2f}s", flush=True)
        if result.stdout:
            safe = result.stdout.encode("ascii", "backslashreplace").decode("ascii")
            print(f"DEBUG: {label} stdout:\n{safe}", flush=True)
        if result.stderr:
            safe = result.stderr.encode("ascii", "backslashreplace").decode("ascii")
            print(f"DEBUG: {label} stderr:\n{safe}", flush=True)
        return result

    try:
        print("DEBUG: starting package uninstall test", flush=True)
        status = _run_logged("dpkg status", "sudo -n dpkg -s xp2p")
        if status.rc != 0:
            print("DEBUG: xp2p not installed; attempting cached install before purge", flush=True)
            install = linux_env.run_guest_script_with_env(
                server_host,
                "scripts/linux/install_xp2p.sh",
                {"XP2P_SKIP_BUILD": "1"},
                timeout=300,
            )
            if install.rc != 0:
                pytest.skip(
                    "xp2p package not installed and cached build not available "
                    f"(exit {install.rc}).\nSTDOUT:\n{install.stdout}\nSTDERR:\n{install.stderr}"
                )
            print("DEBUG: xp2p cached install completed", flush=True)

        _run_logged(
            "remove client config",
            "sudo -n /usr/bin/xp2p client remove --path /etc/xp2p --config-dir config-client --all --ignore-missing --quiet"
        )
        _run_logged(
            "remove server config",
            "sudo -n /usr/bin/xp2p server remove --path /etc/xp2p --config-dir config-server --ignore-missing --quiet"
        )

        _run_logged(
            "remove config/log roots",
            "sudo -n rm -rf /etc/xp2p /var/log/xp2p >/dev/null 2>&1 || true",
        )

        remove_result = _run_logged("dpkg purge", "sudo -n dpkg -P xp2p", timeout=300)
        if remove_result.rc != 0:
            pytest.fail(
                "dpkg purge failed "
                f"(exit {remove_result.rc}).\nSTDOUT:\n{remove_result.stdout}\nSTDERR:\n{remove_result.stderr}"
            )
        _run_logged("systemctl daemon-reload", "sudo -n systemctl daemon-reload")

        status_client = _run_logged("systemctl status client", "sudo -n systemctl status xp2p-client.service")
        status_server = _run_logged("systemctl status server", "sudo -n systemctl status xp2p-server.service")
        assert status_client.rc != 0 and _unit_missing(status_client)
        assert status_server.rc != 0 and _unit_missing(status_server)
        _assert_service_inactive(server_host, "xp2p-client.service")
        _assert_service_inactive(server_host, "xp2p-server.service")

        print("DEBUG: asserting missing install/log/bin/service paths", flush=True)
        _assert_missing(install_root)
        _assert_missing(log_root)
        _assert_missing(bin_path)
        for svc in service_paths:
            _assert_missing(svc)
    finally:
        print("DEBUG: reinstalling xp2p package", flush=True)
        reinstall = linux_env.run_guest_script(server_host, "scripts/linux/install_xp2p.sh", timeout=600)
        if reinstall.rc != 0:
            pytest.fail(
                "Failed to reinstall xp2p package "
                f"(exit {reinstall.rc}).\nSTDOUT:\n{reinstall.stdout}\nSTDERR:\n{reinstall.stderr}"
            )
        print("DEBUG: reinstall completed", flush=True)


@pytest.mark.host
@pytest.mark.linux
def test_package_remove_keeps_config_files(server_host, xp2p_server_runner):
    client_paths = config_files.config_paths(helpers.CLIENT_CONFIG_DIR, config_files.CLIENT_CONFIG_FILES)
    server_paths = config_files.config_paths(helpers.SERVER_CONFIG_DIR, config_files.SERVER_CONFIG_FILES)
    state_paths = helpers.CLIENT_STATE_FILES + helpers.SERVER_STATE_FILES

    def _assert_paths_exist(paths: list[PurePosixPath]) -> None:
        missing = [path for path in paths if not helpers.path_exists(server_host, path)]
        if missing:
            rendered = "\n".join(path.as_posix() for path in missing)
            pytest.fail(f"Expected config files to exist:\n{rendered}")

    for path in state_paths:
        if helpers.path_exists(server_host, path):
            helpers.remove_path(server_host, path)
    try:
        xp2p_server_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.120.10",
            "--user",
            "cfg-client@example.com",
            "--password",
            "cfg-client-secret",
            "--force",
            check=True,
        )
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62135",
            "--host",
            "cfg-server.example.com",
            "--force",
            check=True,
        )

        _assert_paths_exist(client_paths + server_paths)

        remove_result = server_host.run("sudo -n dpkg -r xp2p")
        if remove_result.rc != 0:
            pytest.fail(
                "dpkg remove failed "
                f"(exit {remove_result.rc}).\nSTDOUT:\n{remove_result.stdout}\nSTDERR:\n{remove_result.stderr}"
            )

        _assert_paths_exist(client_paths + server_paths)
    finally:
        reinstall = linux_env.run_guest_script(server_host, "scripts/linux/install_xp2p.sh")
        if reinstall.rc != 0:
            pytest.fail(
                "Failed to reinstall xp2p package "
                f"(exit {reinstall.rc}).\nSTDOUT:\n{reinstall.stdout}\nSTDERR:\n{reinstall.stderr}"
            )
        for path in state_paths:
            if helpers.path_exists(server_host, path):
                helpers.remove_path(server_host, path)
