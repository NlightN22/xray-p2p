from __future__ import annotations

import base64
import json
import time
from contextlib import contextmanager
from pathlib import Path

import pytest

from tests.host.win import env as win_env
from tests.host.win.diagnostics import remote_files
from tests.host.win.flows import apply as apply_flow

CLIENT_DEPLOY_STDOUT = Path(r"C:\Windows\Temp\xp2p-guest-logs\client-deploy.log")
SERVER_DEPLOY_STDOUT = Path(r"C:\Windows\Temp\xp2p-guest-logs\server-deploy.log")
DEPLOY_PORT = "62125"
TROJAN_PORT = "58601"
LOG_WAIT_TIMEOUT = 60
DEPLOY_FIREWALL_RULE = "xp2p-test-deploy-allow"


@contextmanager
def timed(label: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: {label}: {elapsed:.2f}s")


def set_firewall_rule(
    server_host,
    *,
    ensure: str,
    remote_address: str,
    port: int,
    action: str,
) -> None:
    rule_name = f"{DEPLOY_FIREWALL_RULE}-{port}"
    script = f"""
$ErrorActionPreference = 'Stop'
$name = {win_env.ps_quote(rule_name)}
$remote = {win_env.ps_quote(remote_address)}
$port = {port}
$ensure = {win_env.ps_quote(ensure)}
$action = {win_env.ps_quote(action)}
Get-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue | ForEach-Object {{
    Remove-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue
}}
if ($ensure -eq 'Present') {{
    New-NetFirewallRule `
        -DisplayName $name `
        -Direction Inbound `
        -Action $action `
        -Protocol TCP `
        -LocalPort $port `
        -RemoteAddress $remote `
        -Profile Any `
        -EdgeTraversalPolicy Block | Out-Null
}}
exit 0
"""
    result = win_env.run_powershell(server_host, script, label="set_firewall_rule")
    if result.rc != 0:
        pytest.fail(
            f"Failed to set deploy firewall rule Ensure={ensure} Action={action}.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def start_client_deploy(
    host,
    *,
    remote_host: str,
    deploy_port: str,
    trojan_user: str,
    trojan_password: str,
    trojan_port: str,
) -> dict[str, str | int]:
    result = win_env.run_guest_script(
        host,
        "scripts/start_xp2p_client_deploy.ps1",
        Xp2pPath=str(win_env.XP2P_EXE),
        LogPath=str(CLIENT_DEPLOY_STDOUT),
        RemoteHost=remote_host,
        DeployPort=deploy_port,
        TrojanUser=trojan_user,
        TrojanPassword=trojan_password,
        TrojanPort=trojan_port,
    )
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
    return {"pid": int(pid), "stdout": stdout_path, "stderr": stderr_path}


def start_server_deploy(
    host,
    *,
    listen_addr: str,
    deploy_link: str,
) -> dict[str, str | int]:
    return start_server_deploy_with_args(
        host,
        listen_addr=listen_addr,
        deploy_link=deploy_link,
        args=[],
        env_overrides={},
    )


def start_server_deploy_with_args(
    host,
    *,
    listen_addr: str,
    deploy_link: str,
    args: list[str] | None = None,
    env_overrides: dict[str, str] | None = None,
) -> dict[str, str | int]:
    args = [] if args is None else args
    env_overrides = {} if env_overrides is None else env_overrides
    result = win_env.run_guest_script(
        host,
        "scripts/start_xp2p_server_deploy.ps1",
        Xp2pPath=str(win_env.XP2P_EXE),
        LogPath=str(SERVER_DEPLOY_STDOUT),
        ListenAddress=listen_addr,
        DeployLink=deploy_link,
        AdditionalArgsBase64=_encode_args_payload(args),
        EnvOverridesBase64=_encode_env_payload(env_overrides),
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
    return {"pid": int(pid), "stdout": stdout_path, "stderr": stderr_path}


def _encode_args_payload(args: list[str]) -> str:
    return base64.b64encode(json.dumps([str(arg) for arg in args]).encode("utf-8")).decode("ascii")


def _encode_env_payload(env: dict[str, str]) -> str:
    payload = {str(key): str(value) for key, value in env.items()}
    return base64.b64encode(json.dumps(payload).encode("utf-8")).decode("ascii")


def stop_process(host, pid_value: int) -> None:
    script = f"""
$pidValue = {pid_value}
if ($pidValue -le 0) {{
    exit 0
}}
$proc = Get-Process -Id $pidValue -ErrorAction SilentlyContinue
if ($proc) {{
    try {{
        Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue
    }} catch {{ }}
}}
Start-Sleep -Milliseconds 200
$xray = Get-Process -Name xray -ErrorAction SilentlyContinue
if ($xray) {{
    foreach ($item in $xray) {{
        try {{
            Stop-Process -Id $item.Id -Force -ErrorAction SilentlyContinue
        }} catch {{ }}
    }}
}}
exit 0
"""
    win_env.run_powershell(host, script)


def stop_xp2p_processes(host) -> None:
    win_env.stop_xp2p_processes(host)


def stop_listening_ports(host, ports: list[int]) -> None:
    ports_list = ", ".join(str(int(port)) for port in ports)
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$ports = @({ports_list})
foreach ($port in $ports) {{
    $items = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue
    if (-not $items) {{
        continue
    }}
    foreach ($item in $items) {{
        $pid = $item.OwningProcess
        if ($pid) {{
            try {{
                Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
            }} catch {{ }}
        }}
    }}
}}
exit 0
"""
    win_env.run_powershell(host, script, label="stop_listening_ports")


def remove_paths(host, paths: list[Path]) -> None:
    targets = ", ".join(win_env.ps_quote(str(path)) for path in paths)
    script = f"""
$ErrorActionPreference = 'Stop'
$targets = @({targets})
foreach ($target in $targets) {{
    if (-not $target) {{
        continue
    }}
    if (Test-Path $target) {{
        Remove-Item -Path $target -Force -Recurse -ErrorAction SilentlyContinue
    }}
}}
exit 0
"""
    result = win_env.run_powershell(host, script, label="remove_paths")
    if result.rc != 0:
        pytest.fail(
            "Failed to remove remote paths.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def remove_path(host, path: Path) -> None:
    remove_paths(host, [path])


def wait_for_client_link(host, proc_info: dict[str, str | int]) -> str:
    def _extract_link(text: str) -> str | None:
        for line in text.splitlines():
            if "client deploy: link generated" not in line or "link:" not in line:
                continue
            return line.split("link:", 1)[1].strip()
        return None

    link = wait_for_log_value(
        host,
        proc_info,
        extractor=_extract_link,
        description="xp2p client deploy link",
        timeout=LOG_WAIT_TIMEOUT,
    )
    if not link:
        pytest.fail("xp2p client deploy log did not include a deploy link")
    return link


def wait_for_log_phrase(host, proc_info: dict[str, str | int], phrase: str, *, timeout: int) -> None:
    expected_variants = (phrase, f"{phrase}")

    def _matcher(text: str) -> bool | None:
        for variant in expected_variants:
            if variant in text:
                return True
        return None

    wait_for_log_value(host, proc_info, extractor=_matcher, description=f"'{phrase}'", timeout=timeout)


def wait_for_any_log_phrase(
    host,
    proc_info: dict[str, str | int],
    phrases: list[str],
    *,
    timeout: int,
) -> str:
    expected_variants = [(phrase, f"{phrase}") for phrase in phrases]

    def _matcher(text: str) -> str | None:
        for phrase, prefixed in expected_variants:
            if phrase in text or prefixed in text:
                return phrase
        return None

    return wait_for_log_value(
        host,
        proc_info,
        extractor=_matcher,
        description=f"any of {phrases}",
        timeout=timeout,
    )


def wait_for_apply_request_clear(host, *, timeout: float = 60.0) -> None:
    apply_flow.wait_for_apply_request_clear(host, timeout=timeout, dump_label="timeout-apply-request")


def wait_for_log_value(
    host,
    proc_info: dict[str, str | int],
    *,
    extractor,
    description: str,
    timeout: int,
):
    deadline = time.time() + timeout
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        stdout_text = remote_files.read_optional_text(host, proc_info["stdout"])
        stderr_text = remote_files.read_optional_text(host, proc_info["stderr"])
        combined = "\n".join(filter(None, [stdout_text, stderr_text]))
        value = extractor(combined)
        if value:
            return value
        last_stdout = stdout_text or last_stdout
        last_stderr = stderr_text or last_stderr
        time.sleep(1)
    stdout_tail = "\n".join((last_stdout or "").splitlines()[-30:])
    stderr_tail = "\n".join((last_stderr or "").splitlines()[-30:])
    dump_path = win_env.dump_failure_state(host, label=f"timeout-{description}")
    pytest.fail(
        f"Timed out waiting for {description}.\nFailure dump: {dump_path}\n"
        f"STDOUT tail:\n{stdout_tail}\nSTDERR tail:\n{stderr_tail}"
    )


def wait_for_ping_ok_or_server_failure(
    client_host,
    server_host,
    client_proc: dict[str, str | int],
    server_proc: dict[str, str | int],
    *,
    timeout: int,
) -> None:
    deadline = time.time() + timeout
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        stdout_text = remote_files.read_optional_text(client_host, client_proc["stdout"])
        stderr_text = remote_files.read_optional_text(client_host, client_proc["stderr"])
        combined = "\n".join(filter(None, [stdout_text, stderr_text]))
        if "client deploy: ping ok" in combined or "client deploy: ping ok" in combined:
            return
        last_stdout = stdout_text or last_stdout
        last_stderr = stderr_text or last_stderr
        server_logs = read_combined_logs(server_host, server_proc)
        if "server deploy: xray-core start failed" in server_logs:
            dump_path = win_env.dump_failure_state(server_host, label="server-deploy-xray-failed")
            pytest.fail(
                "Server deploy xray-core failed while waiting for client ping.\n"
                f"Server logs:\n{server_logs}\n\nFailure dump: {dump_path}"
            )
        if "server deploy: stopped" in server_logs:
            if not service_running(server_host, "xp2p-server"):
                dump_path = win_env.dump_failure_state(server_host, label="server-deploy-stopped")
                pytest.fail(
                    "Server deploy stopped while waiting for client ping.\n"
                    f"Server logs:\n{server_logs}\n\nFailure dump: {dump_path}"
                )
        time.sleep(1)
    stdout_tail = "\n".join((last_stdout or "").splitlines()[-30:])
    stderr_tail = "\n".join((last_stderr or "").splitlines()[-30:])
    dump_path = win_env.dump_failure_state(client_host, label="client-deploy-ping-timeout")
    pytest.fail(
        "Timed out waiting for 'client deploy: ping ok'.\n"
        f"Failure dump: {dump_path}\nSTDOUT tail:\n{stdout_tail}\nSTDERR tail:\n{stderr_tail}"
    )


def read_combined_logs(host, proc_info: dict[str, str | int]) -> str:
    stdout_text = remote_files.read_optional_text(host, proc_info["stdout"])
    stderr_text = remote_files.read_optional_text(host, proc_info["stderr"])
    return "\n".join(filter(None, [stdout_text, stderr_text]))


def service_running(host, service_name: str) -> bool:
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$svc = Get-Service -Name {win_env.ps_quote(service_name)} -ErrorAction SilentlyContinue
if (-not $svc) {{
    exit 3
}}
if ($svc.Status -eq 'Running') {{
    exit 0
}}
exit 4
"""
    result = win_env.run_powershell(host, script, label="service_running")
    if result.rc == 0:
        return True
    if result.rc in (3, 4):
        return False
    return False


def extract_marker(output: str | None, marker: str) -> str | None:
    for raw in (output or "").splitlines():
        line = raw.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None
