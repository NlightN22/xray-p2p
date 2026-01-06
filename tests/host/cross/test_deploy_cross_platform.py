from __future__ import annotations

import time
from collections.abc import Iterator
from pathlib import Path, PurePosixPath
from typing import TypedDict
from urllib import parse

import pytest
from testinfra.host import Host

from tests.host.linux import _helpers as linux_helpers
from tests.host.linux import env as linux_env
from tests.host.win import env as win_env

pytestmark = [pytest.mark.host, pytest.mark.cross]

DEPLOY_PORT = "62125"
TROJAN_PORT = "58601"
LOG_WAIT_TIMEOUT = 20

LINUX_ARTIFACT_ROOT = PurePosixPath("/srv/xray-p2p/build/artifacts/deploy")
WINDOWS_ARTIFACT_ROOT = Path(r"C:\xp2p\build\artifacts\deploy")

LINUX_CLIENT_LOG = LINUX_ARTIFACT_ROOT / "linux-client-deploy.log"
LINUX_SERVER_LOG = LINUX_ARTIFACT_ROOT / "linux-server-deploy.log"
WINDOWS_CLIENT_LOG = WINDOWS_ARTIFACT_ROOT / "windows-client-deploy.log"
WINDOWS_SERVER_LOG = WINDOWS_ARTIFACT_ROOT / "windows-server-deploy.log"

DEFAULT_WINDOWS_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
DEFAULT_LINUX_INSTALL_DIR = PurePosixPath("/etc/xp2p")
WINDOWS_HEARTBEAT_STATE_FILE = DEFAULT_WINDOWS_INSTALL_DIR / "state-heartbeat.json"


class WindowsProcInfo(TypedDict):
    pid: int
    stdout: Path
    stderr: Path


@pytest.fixture(scope="session")
def linux_hosts() -> dict[str, Host]:
    linux_env.require_vagrant_environment()
    factory = linux_env.machine_host_factory()
    client = factory(linux_env.DEFAULT_CLIENT)
    server = factory(linux_env.DEFAULT_SERVER)
    linux_env.ensure_xp2p_installed(linux_env.DEFAULT_CLIENT, client)
    linux_env.ensure_xp2p_installed(linux_env.DEFAULT_SERVER, server)
    return {"client": client, "server": server}


@pytest.fixture(scope="session")
def windows_hosts() -> Iterator[dict[str, Host]]:
    win_env.require_vagrant_environment()
    server = win_env.get_ssh_host(win_env.DEFAULT_SERVER)
    client = win_env.get_ssh_host(win_env.DEFAULT_CLIENT)
    for host in (server, client):
        win_env.ensure_admin_token(host)
        win_env.ensure_program_files_install(host, force_reinstall=True)
    yield {"client": client, "server": server}
    msi_path = win_env.ensure_msi_package(server)
    win_env.uninstall_xp2p_from_msi(server, msi_path)
    win_env.uninstall_xp2p_from_msi(client, msi_path)


def _linux_runner(host: Host):
    def _runner(*args: str, check: bool = False):
        cmd = list(args)
        if len(cmd) >= 2 and cmd[0] in {"client", "server"} and cmd[1] == "remove":
            if "--quiet" not in cmd:
                cmd.append("--quiet")
        result = linux_env.run_xp2p(host, *cmd)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner


def _windows_runner(host: Host):
    def _runner(*args: str, check: bool = False):
        cmd = list(args)
        if len(cmd) >= 2 and cmd[0] in {"client", "server"} and cmd[1] == "remove":
            if "--quiet" not in cmd:
                cmd.append("--quiet")
        result = win_env.run_xp2p(host, cmd)
        stdout = result.stdout or ""
        if "__XP2P_MISSING__" in stdout:
            pytest.skip(
                f"xp2p.exe not found on {host.backend.hostname} at {win_env.XP2P_EXE}. "
                "Provision the guest before running host tests."
            )
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner


def _cleanup_windows_server_install(host: Host, runner, install_dir: Path | None = None) -> None:
    args = ["server", "remove", "--ignore-missing", "--quiet"]
    if install_dir is not None:
        args.extend(["--path", str(install_dir)])
    runner(*args)


def _cleanup_windows_client_install(host: Host, runner, install_dir: Path | None = None) -> None:
    args = ["client", "remove", "--all", "--ignore-missing", "--quiet"]
    if install_dir is not None:
        args.extend(["--path", str(install_dir)])
    runner(*args)


def _start_linux_client_deploy(
    host: Host,
    *,
    log_path: PurePosixPath,
    remote_host: str,
    deploy_port: str,
    trojan_user: str,
    trojan_password: str,
    trojan_port: str,
    install_dir: str | None = None,
) -> int:
    args = [
        log_path.as_posix(),
        remote_host,
        deploy_port,
        trojan_user,
        trojan_password,
        trojan_port,
    ]
    if install_dir:
        args += ["--install-dir", install_dir]
    result = linux_env.run_guest_script(host, "scripts/linux/start_xp2p_client_deploy.sh", *args)
    if result.rc != 0:
        pytest.fail(
            "Failed to start xp2p client deploy.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid = _extract_marker(result.stdout, "__XP2P_PID__=")
    if not pid:
        pytest.fail(
            "xp2p client deploy script did not emit PID marker.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return int(pid)


def _start_linux_server_deploy(
    host: Host,
    *,
    log_path: PurePosixPath,
    listen_addr: str,
    deploy_link: str,
) -> int:
    result = linux_env.run_guest_script(
        host,
        "scripts/linux/start_xp2p_server_deploy.sh",
        log_path.as_posix(),
        listen_addr,
        deploy_link,
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to start xp2p server deploy.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid = _extract_marker(result.stdout, "__XP2P_PID__=")
    if not pid:
        pytest.fail(
            "xp2p server deploy script did not emit PID marker.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return int(pid)


def _start_windows_client_deploy(
    host: Host,
    *,
    log_path: Path,
    remote_host: str,
    deploy_port: str,
    trojan_user: str,
    trojan_password: str,
    trojan_port: str,
    install_dir: str | None = None,
) -> WindowsProcInfo:
    parameters: dict[str, object] = {
        "Xp2pPath": str(win_env.XP2P_EXE),
        "LogPath": str(log_path),
        "RemoteHost": remote_host,
        "DeployPort": deploy_port,
        "TrojanUser": trojan_user,
        "TrojanPassword": trojan_password,
        "TrojanPort": trojan_port,
    }
    if install_dir:
        parameters["InstallDir"] = install_dir
    result = win_env.run_guest_script(host, "scripts/start_xp2p_client_deploy.ps1", **parameters)
    if result.rc != 0:
        pytest.fail(
            "Failed to start xp2p client deploy.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid = _extract_marker(result.stdout, "PID=")
    stdout_path = _extract_marker(result.stdout, "STDOUT=")
    stderr_path = _extract_marker(result.stdout, "STDERR=")
    if not pid or not stdout_path or not stderr_path:
        pytest.fail(
            "xp2p client deploy script did not emit expected markers.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return {"pid": int(pid), "stdout": Path(stdout_path), "stderr": Path(stderr_path)}


def _start_windows_server_deploy(
    host: Host,
    *,
    log_path: Path,
    listen_addr: str,
    deploy_link: str,
) -> WindowsProcInfo:
    result = win_env.run_guest_script(
        host,
        "scripts/start_xp2p_server_deploy.ps1",
        Xp2pPath=str(win_env.XP2P_EXE),
        LogPath=str(log_path),
        ListenAddress=listen_addr,
        DeployLink=deploy_link,
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to start xp2p server deploy.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid = _extract_marker(result.stdout, "PID=")
    stdout_path = _extract_marker(result.stdout, "STDOUT=")
    stderr_path = _extract_marker(result.stdout, "STDERR=")
    if not pid or not stdout_path or not stderr_path:
        pytest.fail(
            "xp2p server deploy script did not emit expected markers.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return {"pid": int(pid), "stdout": Path(stdout_path), "stderr": Path(stderr_path)}


def _stop_windows_process(host: Host, pid: int) -> None:
    script = f"""
$ErrorActionPreference = 'Stop'
$proc = Get-Process -Id {pid} -ErrorAction SilentlyContinue
if ($proc) {{
    Stop-Process -Id $proc.Id -Force
}}
exit 0
"""
    win_env.run_powershell(host, script)


def _read_optional_linux_log(host: Host, path: PurePosixPath) -> str:
    result = linux_env.run_guest_script(host, "scripts/linux/read_file.sh", path.as_posix())
    if result.rc == 0:
        return result.stdout or ""
    if result.rc == 3:
        return ""
    pytest.fail(
        f"Failed to read log {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def _read_optional_windows_text(host: Host, path: Path) -> str:
    if not win_env.path_exists(host, path):
        return ""
    return win_env.read_text(host, path)


def _read_combined_windows_logs(host: Host, proc_info: WindowsProcInfo) -> str:
    stdout_text = _read_optional_windows_text(host, proc_info["stdout"])
    stderr_text = _read_optional_windows_text(host, proc_info["stderr"])
    return "\n".join(filter(None, [stdout_text, stderr_text]))


def _wait_for_log_value_linux(
    host: Host,
    path: PurePosixPath,
    *,
    extractor,
    description: str,
    timeout: int,
):
    deadline = time.time() + timeout
    last_text = ""
    while time.time() < deadline:
        text = _read_optional_linux_log(host, path)
        if text:
            value = extractor(text)
            if value:
                return value
            last_text = text
        time.sleep(1)
    tail = "\n".join((last_text or "").splitlines()[-30:])
    pytest.fail(f"Timed out waiting for {description}. Recent log tail:\n{tail}")


def _wait_for_log_value_windows(
    host: Host,
    proc_info: WindowsProcInfo,
    *,
    extractor,
    description: str,
    timeout: int,
):
    deadline = time.time() + timeout
    last_text = ""
    while time.time() < deadline:
        text = _read_combined_windows_logs(host, proc_info)
        if text:
            value = extractor(text)
            if value:
                return value
            last_text = text
        time.sleep(1)
    tail = "\n".join((last_text or "").splitlines()[-30:])
    pytest.fail(f"Timed out waiting for {description}. Recent log tail:\n{tail}")


def _wait_for_log_phrase_linux(host: Host, path: PurePosixPath, phrase: str, *, timeout: int) -> None:
    expected_variants = (phrase, f"xp2p: {phrase}")

    def _matcher(text: str) -> bool | None:
        for variant in expected_variants:
            if variant in text:
                return True
        return None

    _wait_for_log_value_linux(
        host,
        path,
        extractor=_matcher,
        description=f"'{phrase}' in {path}",
        timeout=timeout,
    )


def _wait_for_log_phrase_windows(host: Host, proc_info: WindowsProcInfo, phrase: str, *, timeout: int) -> None:
    expected_variants = (phrase, f"xp2p: {phrase}")

    def _matcher(text: str) -> bool | None:
        for variant in expected_variants:
            if variant in text:
                return True
        return None

    _wait_for_log_value_windows(
        host,
        proc_info,
        extractor=_matcher,
        description=f"'{phrase}'",
        timeout=timeout,
    )


def _wait_for_client_link_linux(host: Host, log_path: PurePosixPath) -> str:
    def _extract_link(text: str) -> str | None:
        for line in text.splitlines():
            if "client deploy: link generated" not in line:
                continue
            if "link:" not in line:
                continue
            return line.split("link:", 1)[1].strip()
        return None

    link = _wait_for_log_value_linux(
        host,
        log_path,
        extractor=_extract_link,
        description="xp2p client deploy link",
        timeout=LOG_WAIT_TIMEOUT,
    )
    if not link:
        pytest.fail("xp2p client deploy log did not include a deploy link")
    return link


def _wait_for_client_link_windows(host: Host, proc_info: WindowsProcInfo) -> str:
    def _extract_link(text: str) -> str | None:
        for line in text.splitlines():
            if "client deploy: link generated" not in line:
                continue
            if "link:" not in line:
                continue
            return line.split("link:", 1)[1].strip()
        return None

    link = _wait_for_log_value_windows(
        host,
        proc_info,
        extractor=_extract_link,
        description="xp2p client deploy link",
        timeout=LOG_WAIT_TIMEOUT,
    )
    if not link:
        pytest.fail("xp2p client deploy log did not include a deploy link")
    return link


def _assert_link_install_dir(link: str, expected_install_dir: str | None) -> None:
    parsed = parse.urlparse(link)
    query = parse.parse_qs(parsed.query)
    if expected_install_dir is None:
        assert "install_dir" not in query, f"install_dir should be omitted in link: {query}"
        return
    assert query.get("install_dir") == [expected_install_dir], f"install_dir mismatch: {query}"


def _detect_windows_ipv4(host: Host) -> str:
    script = """
$ErrorActionPreference = 'Stop'
$addresses = Get-NetIPAddress -AddressFamily IPv4 -PrefixOrigin (@('Dhcp', 'Manual')) `
    | Where-Object { $_.IPAddress -ne '127.0.0.1' } `
    | Select-Object -ExpandProperty IPAddress
if (-not $addresses) {
    exit 3
}
$addresses
"""
    result = win_env.run_powershell(host, script)
    if result.rc != 0:
        pytest.fail(
            "Failed to detect IPv4 addresses.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not addresses:
        pytest.fail("No IPv4 addresses found on host")
    for addr in addresses:
        if not addr.startswith("10.0.2."):
            return addr
    return addresses[0]


def _detect_linux_ipv4_non_nat(host: Host) -> str:
    command = "ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1"
    result = host.run(command)
    if result.rc != 0:
        pytest.fail(
            "Failed to detect IPv4 addresses.\n"
            f"CMD: {command}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not addresses:
        pytest.fail("No IPv4 addresses found on host")
    for addr in addresses:
        if not addr.startswith("10.0.2."):
            return addr
    return addresses[0]


def _extract_marker(output: str | None, marker: str) -> str | None:
    for raw in (output or "").splitlines():
        line = raw.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None


def _reset_linux_logs(host: Host, path: PurePosixPath) -> None:
    linux_helpers.remove_path(host, path)


def _reset_windows_logs(host: Host, path: Path) -> None:
    win_env.remove_path(host, path)
    win_env.remove_path(host, Path(str(path) + ".err"))


def _assert_windows_server_install_dir(host: Host, install_dir: Path) -> None:
    state_path = install_dir / "install-state-server.json"
    config_dir = install_dir / linux_helpers.SERVER_CONFIG_DIR_NAME
    assert win_env.path_exists(host, state_path), f"server install state missing: {state_path}"
    assert win_env.path_exists(host, config_dir), f"server config dir missing: {config_dir}"


def _assert_linux_server_install_dir(host: Host, install_dir: PurePosixPath) -> None:
    state_path = install_dir / "install-state-server.json"
    config_dir = install_dir / linux_helpers.SERVER_CONFIG_DIR_NAME
    assert linux_helpers.path_exists(host, state_path), f"server install state missing: {state_path}"
    assert linux_helpers.path_exists(host, config_dir), f"server config dir missing: {config_dir}"


def _wait_for_error_phrase_linux(host: Host, path: PurePosixPath, phrase: str) -> None:
    def _matcher(text: str) -> bool | None:
        if phrase in text:
            return True
        return None

    _wait_for_log_value_linux(
        host,
        path,
        extractor=_matcher,
        description=f"'{phrase}' in {path}",
        timeout=LOG_WAIT_TIMEOUT,
    )


def _wait_for_error_phrase_windows(host: Host, proc_info: WindowsProcInfo, phrase: str) -> None:
    def _matcher(text: str) -> bool | None:
        if phrase in text:
            return True
        return None

    _wait_for_log_value_windows(
        host,
        proc_info,
        extractor=_matcher,
        description=f"'{phrase}'",
        timeout=LOG_WAIT_TIMEOUT,
    )


def _assert_windows_tcp_reachable(host: Host, target: str, port: int, *, timeout_seconds: int = 3) -> None:
    target_quoted = win_env.ps_quote(target)
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target_quoted}
$port = {port}
$timeoutMs = {timeout_seconds * 1000}
$client = New-Object System.Net.Sockets.TcpClient
try {{
    $async = $client.BeginConnect($target, $port, $null, $null)
    if (-not $async.AsyncWaitHandle.WaitOne($timeoutMs)) {{
        exit 3
    }}
    $client.EndConnect($async)
    exit 0
}} catch {{
    exit 4
}} finally {{
    $client.Close()
}}
"""
    result = win_env.run_powershell(host, script)
    if result.rc != 0:
        pytest.fail(
            "Windows client cannot reach Linux server deploy port.\n"
            f"Target: {target}:{port}\n"
            f"RC: {result.rc}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.cross
def test_cross_deploy_linux_client_windows_server(linux_hosts, windows_hosts):
    client_host = linux_hosts["client"]
    server_host = windows_hosts["server"]
    linux_client_runner = _linux_runner(client_host)
    win_server_runner = _windows_runner(server_host)

    linux_helpers.cleanup_client_install(client_host, linux_client_runner)
    _cleanup_windows_server_install(server_host, win_server_runner, DEFAULT_WINDOWS_INSTALL_DIR)
    linux_helpers.remove_path(client_host, linux_helpers.HEARTBEAT_STATE_FILE)
    win_env.remove_path(server_host, WINDOWS_HEARTBEAT_STATE_FILE)

    server_ip = _detect_windows_ipv4(server_host)
    trojan_user = "deploy-cross@example.com"
    trojan_password = "deploy-cross-pass"

    def _run_scenario(install_dir: str | None, expect_error: bool) -> None:
        client_pid = None
        server_proc: WindowsProcInfo | None = None
        success = False
        _reset_linux_logs(client_host, LINUX_CLIENT_LOG)
        _reset_windows_logs(server_host, WINDOWS_SERVER_LOG)
        try:
            client_pid = _start_linux_client_deploy(
                client_host,
                log_path=LINUX_CLIENT_LOG,
                remote_host=server_ip,
                deploy_port=DEPLOY_PORT,
                trojan_user=trojan_user,
                trojan_password=trojan_password,
                trojan_port=TROJAN_PORT,
                install_dir=install_dir,
            )
            link = _wait_for_client_link_linux(client_host, LINUX_CLIENT_LOG)
            _assert_link_install_dir(link, install_dir)

            server_proc = _start_windows_server_deploy(
                server_host,
                log_path=WINDOWS_SERVER_LOG,
                listen_addr=f":{DEPLOY_PORT}",
                deploy_link=link,
            )

            if expect_error:
                _wait_for_error_phrase_linux(client_host, LINUX_CLIENT_LOG, "server rejected deploy request")
                _wait_for_error_phrase_linux(client_host, LINUX_CLIENT_LOG, "invalid install_dir for Windows")
                success = True
                return

            _wait_for_log_phrase_windows(
                server_host,
                server_proc,
                "server deploy: manifest decrypted",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_windows(
                server_host,
                server_proc,
                "server deploy: starting xray-core",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_linux(
                client_host,
                LINUX_CLIENT_LOG,
                "client deploy: trojan link received",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_linux(
                client_host,
                LINUX_CLIENT_LOG,
                "client deploy: local install completed",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_linux(
                client_host,
                LINUX_CLIENT_LOG,
                "client deploy: ping ok",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_linux(
                client_host,
                LINUX_CLIENT_LOG,
                "client deploy: client run active",
                timeout=LOG_WAIT_TIMEOUT,
            )

            _assert_windows_server_install_dir(server_host, DEFAULT_WINDOWS_INSTALL_DIR)
            success = True
        finally:
            if client_pid:
                linux_env.run_guest_script(client_host, "scripts/linux/stop_process.sh", str(client_pid))
            if server_proc:
                _stop_windows_process(server_host, int(server_proc["pid"]))
            linux_helpers.cleanup_client_install(client_host, linux_client_runner)
            _cleanup_windows_server_install(server_host, win_server_runner, DEFAULT_WINDOWS_INSTALL_DIR)
            linux_helpers.remove_path(client_host, linux_helpers.HEARTBEAT_STATE_FILE)
            win_env.remove_path(server_host, WINDOWS_HEARTBEAT_STATE_FILE)
            if success:
                _reset_linux_logs(client_host, LINUX_CLIENT_LOG)
                _reset_windows_logs(server_host, WINDOWS_SERVER_LOG)

    _run_scenario(install_dir=None, expect_error=False)
    _run_scenario(install_dir=str(DEFAULT_WINDOWS_INSTALL_DIR), expect_error=False)
    _run_scenario(install_dir="/invalid/path", expect_error=True)


@pytest.mark.host
@pytest.mark.cross
def test_cross_deploy_windows_client_linux_server(linux_hosts, windows_hosts):
    client_host = windows_hosts["client"]
    server_host = linux_hosts["server"]
    win_client_runner = _windows_runner(client_host)
    linux_server_runner = _linux_runner(server_host)

    _cleanup_windows_client_install(client_host, win_client_runner, DEFAULT_WINDOWS_INSTALL_DIR)
    linux_helpers.cleanup_server_install(server_host, linux_server_runner)
    win_env.remove_path(client_host, WINDOWS_HEARTBEAT_STATE_FILE)
    linux_helpers.remove_path(server_host, linux_helpers.HEARTBEAT_STATE_FILE)

    server_ip = _detect_linux_ipv4_non_nat(server_host)
    trojan_user = "deploy-cross@example.com"
    trojan_password = "deploy-cross-pass"

    def _run_scenario(install_dir: str | None) -> None:
        client_proc: WindowsProcInfo | None = None
        server_pid: int | None = None
        success = False
        _reset_windows_logs(client_host, WINDOWS_CLIENT_LOG)
        _reset_linux_logs(server_host, LINUX_SERVER_LOG)
        try:
            client_proc = _start_windows_client_deploy(
                client_host,
                log_path=WINDOWS_CLIENT_LOG,
                remote_host=server_ip,
                deploy_port=DEPLOY_PORT,
                trojan_user=trojan_user,
                trojan_password=trojan_password,
                trojan_port=TROJAN_PORT,
                install_dir=install_dir,
            )
            link = _wait_for_client_link_windows(client_host, client_proc)
            _assert_link_install_dir(link, install_dir)

            server_pid = _start_linux_server_deploy(
                server_host,
                log_path=LINUX_SERVER_LOG,
                listen_addr=f":{DEPLOY_PORT}",
                deploy_link=link,
            )
            _wait_for_log_phrase_linux(
                server_host,
                LINUX_SERVER_LOG,
                "server deploy: starting listener",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _assert_windows_tcp_reachable(client_host, server_ip, int(DEPLOY_PORT))

            _wait_for_log_phrase_linux(
                server_host,
                LINUX_SERVER_LOG,
                "server deploy: manifest decrypted",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_linux(
                server_host,
                LINUX_SERVER_LOG,
                "server deploy: starting xray-core",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_windows(
                client_host,
                client_proc,
                "client deploy: trojan link received",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_windows(
                client_host,
                client_proc,
                "client deploy: local install completed",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_windows(
                client_host,
                client_proc,
                "client deploy: ping ok",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase_windows(
                client_host,
                client_proc,
                "client deploy: client run active",
                timeout=LOG_WAIT_TIMEOUT,
            )

            _assert_linux_server_install_dir(server_host, DEFAULT_LINUX_INSTALL_DIR)
            success = True
        finally:
            if client_proc:
                _stop_windows_process(client_host, int(client_proc["pid"]))
            if server_pid:
                linux_env.run_guest_script(server_host, "scripts/linux/stop_process.sh", str(server_pid))
            _cleanup_windows_client_install(client_host, win_client_runner, DEFAULT_WINDOWS_INSTALL_DIR)
            linux_helpers.cleanup_server_install(server_host, linux_server_runner)
            win_env.remove_path(client_host, WINDOWS_HEARTBEAT_STATE_FILE)
            linux_helpers.remove_path(server_host, linux_helpers.HEARTBEAT_STATE_FILE)
            if success:
                _reset_windows_logs(client_host, WINDOWS_CLIENT_LOG)
                _reset_linux_logs(server_host, LINUX_SERVER_LOG)

    _run_scenario(install_dir=None)
    _run_scenario(install_dir=str(DEFAULT_LINUX_INSTALL_DIR))
