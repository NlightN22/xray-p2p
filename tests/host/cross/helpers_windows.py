from __future__ import annotations

import time
from pathlib import Path
from typing import TypedDict

import pytest
from paramiko.ssh_exception import SSHException
from testinfra.host import Host

from tests.host.linux import _helpers as linux_helpers
from tests.host.win import env as win_env
from tests.host.cross.helpers_common import extract_marker


class WindowsProcInfo(TypedDict):
    pid: int
    stdout: Path
    stderr: Path


def wait_for_windows_ssh(host: Host, *, timeout_seconds: int = 60) -> None:
    deadline = time.time() + timeout_seconds
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            result = host.run("cmd /c echo ready")
            if result.rc == 0:
                return
        except SSHException as exc:
            last_error = exc
        time.sleep(2)
    if last_error:
        pytest.fail(f"SSH not ready for Windows host {host.backend.hostname}: {last_error}")
    pytest.fail(f"SSH not ready for Windows host {host.backend.hostname}")


def ensure_windows_ready(host: Host) -> None:
    wait_for_windows_ssh(host)
    last_error: Exception | None = None
    for _ in range(3):
        try:
            win_env.ensure_admin_token(host)
            win_env.ensure_program_files_install(host, force_reinstall=True)
            return
        except SSHException as exc:
            last_error = exc
            time.sleep(2)
    if last_error:
        pytest.fail(f"Failed to prepare Windows host {host.backend.hostname}: {last_error}")
    pytest.fail(f"Failed to prepare Windows host {host.backend.hostname}")


def windows_runner(host: Host):
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


def cleanup_windows_server_install(host: Host, runner, install_dir: Path | None = None) -> None:
    args = ["server", "remove", "--ignore-missing", "--quiet"]
    if install_dir is not None:
        args.extend(["--path", str(install_dir)])
    runner(*args)


def cleanup_windows_client_install(host: Host, runner, install_dir: Path | None = None) -> None:
    args = ["client", "remove", "--all", "--ignore-missing", "--quiet"]
    if install_dir is not None:
        args.extend(["--path", str(install_dir)])
    runner(*args)


def start_windows_client_deploy(
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
    pid = extract_marker(result.stdout, "PID=")
    stdout_path = extract_marker(result.stdout, "STDOUT=")
    stderr_path = extract_marker(result.stdout, "STDERR=")
    if not pid or not stdout_path or not stderr_path:
        pytest.fail(
            "xp2p client deploy script did not emit expected markers.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return {"pid": int(pid), "stdout": Path(stdout_path), "stderr": Path(stderr_path)}


def start_windows_server_deploy(
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
    pid = extract_marker(result.stdout, "PID=")
    stdout_path = extract_marker(result.stdout, "STDOUT=")
    stderr_path = extract_marker(result.stdout, "STDERR=")
    if not pid or not stdout_path or not stderr_path:
        pytest.fail(
            "xp2p server deploy script did not emit expected markers.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return {"pid": int(pid), "stdout": Path(stdout_path), "stderr": Path(stderr_path)}


def stop_windows_process(host: Host, pid: int) -> None:
    script = f"""
$ErrorActionPreference = 'Stop'
$proc = Get-Process -Id {pid} -ErrorAction SilentlyContinue
if ($proc) {{
    Stop-Process -Id $proc.Id -Force
}}
exit 0
"""
    win_env.run_powershell(host, script)


def read_optional_windows_text(host: Host, path: Path) -> str:
    if not win_env.path_exists(host, path):
        return ""
    return win_env.read_text(host, path)


def read_combined_windows_logs(host: Host, proc_info: WindowsProcInfo) -> str:
    stdout_text = read_optional_windows_text(host, proc_info["stdout"])
    stderr_text = read_optional_windows_text(host, proc_info["stderr"])
    return "\n".join(filter(None, [stdout_text, stderr_text]))


def wait_for_log_value_windows(
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
        text = read_combined_windows_logs(host, proc_info)
        if text:
            value = extractor(text)
            if value:
                return value
            last_text = text
        time.sleep(1)
    tail = "\n".join((last_text or "").splitlines()[-30:])
    pytest.fail(f"Timed out waiting for {description}. Recent log tail:\n{tail}")


def wait_for_log_phrase_windows(host: Host, proc_info: WindowsProcInfo, phrase: str, *, timeout: int) -> None:
    expected_variants = (phrase, f"xp2p: {phrase}")

    def _matcher(text: str) -> bool | None:
        for variant in expected_variants:
            if variant in text:
                return True
        return None

    wait_for_log_value_windows(
        host,
        proc_info,
        extractor=_matcher,
        description=f"'{phrase}'",
        timeout=timeout,
    )


def wait_for_client_link_windows(host: Host, proc_info: WindowsProcInfo, *, timeout: int) -> str:
    def _extract_link(text: str) -> str | None:
        for line in text.splitlines():
            if "client deploy: link generated" not in line:
                continue
            if "link:" not in line:
                continue
            return line.split("link:", 1)[1].strip()
        return None

    link = wait_for_log_value_windows(
        host,
        proc_info,
        extractor=_extract_link,
        description="xp2p client deploy link",
        timeout=timeout,
    )
    if not link:
        pytest.fail("xp2p client deploy log did not include a deploy link")
    return link


def wait_for_error_phrase_windows(host: Host, proc_info: WindowsProcInfo, phrase: str, *, timeout: int) -> None:
    def _matcher(text: str) -> bool | None:
        if phrase in text:
            return True
        return None

    wait_for_log_value_windows(
        host,
        proc_info,
        extractor=_matcher,
        description=f"'{phrase}'",
        timeout=timeout,
    )


def detect_windows_ipv4(host: Host) -> str:
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


def reset_windows_logs(host: Host, path: Path) -> None:
    win_env.remove_path(host, path)
    win_env.remove_path(host, Path(str(path) + ".err"))


def assert_windows_server_install_dir(host: Host, install_dir: Path) -> None:
    state_path = win_env.CONFIG_ROOT / "xp2p-server.state.json"
    config_dir = win_env.CONFIG_ROOT / linux_helpers.SERVER_CONFIG_DIR_NAME
    assert win_env.path_exists(host, state_path), f"server install state missing: {state_path}"
    assert win_env.path_exists(host, config_dir), f"server config dir missing: {config_dir}"


def assert_windows_tcp_reachable(host: Host, target: str, port: int, *, timeout_seconds: int = 3) -> None:
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
