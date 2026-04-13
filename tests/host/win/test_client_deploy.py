from __future__ import annotations

import base64
import json
import time
from contextlib import contextmanager
from pathlib import Path

import pytest

from tests.host.win import env as win_env

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = win_env.CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
CLIENT_CONFIG_OUTBOUNDS = CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_ROUTING_JSON = CLIENT_CONFIG_DIR / "routing.json"
CLIENT_CONFIG_FILE = win_env.CONFIG_ROOT / "xp2p-client.toml"
CLIENT_APPLIED_STATE_FILE = win_env.CONFIG_ROOT / "xp2p-client.state.json"
CLIENT_STATE_FILES = [
    CLIENT_CONFIG_FILE,
    CLIENT_APPLIED_STATE_FILE,
]
SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = win_env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
SERVER_INBOUNDS = SERVER_CONFIG_DIR / "inbounds.json"
SERVER_CERT_DEST = SERVER_CONFIG_DIR / "cert.pem"
SERVER_KEY_DEST = SERVER_CONFIG_DIR / "key.pem"
SERVER_CONFIG_FILE = win_env.CONFIG_ROOT / "xp2p-server.toml"
SERVER_APPLIED_STATE_FILE = win_env.CONFIG_ROOT / "xp2p-server.state.json"
SERVER_STATE_FILES = [
    SERVER_CONFIG_FILE,
    SERVER_APPLIED_STATE_FILE,
]
HEARTBEAT_STATE_FILES = [
    win_env.CONFIG_ROOT / "state-heartbeat-client.json",
    win_env.CONFIG_ROOT / "state-heartbeat.json",
]
CLIENT_DEPLOY_STDOUT = Path(r"C:\Windows\Temp\xp2p-guest-logs\client-deploy.log")
SERVER_DEPLOY_STDOUT = Path(r"C:\Windows\Temp\xp2p-guest-logs\server-deploy.log")
DEPLOY_PORT = "62125"
TROJAN_PORT = "58601"
LOG_WAIT_TIMEOUT = 60
DEPLOY_FIREWALL_RULE = "xp2p-test-deploy-allow"


@pytest.mark.host
@pytest.mark.win
def test_windows_client_deploy_end_to_end(
    client_host,
    server_host,
    client_host_ipv4,
    server_host_ipv4,
    xp2p_client_runner,
    xp2p_server_runner,
    xp2p_msi_path,
):
    test_start = time.perf_counter()
    with _timed("cleanup xp2p processes (client)"):
        _stop_xp2p_processes(client_host)
    with _timed("cleanup xp2p processes (server)"):
        _stop_xp2p_processes(server_host)
    with _timed("cleanup client socks listeners"):
        _stop_listening_ports(client_host, [51080, 51180])
    with _timed("cleanup server socks listeners"):
        _stop_listening_ports(server_host, [51080, 51180])
    with _timed("xp2p client remove"):
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
    with _timed("remove client config/state"):
        _remove_paths(client_host, [CLIENT_CONFIG_DIR, *CLIENT_STATE_FILES])
    with _timed("remove server config/state"):
        _remove_paths(server_host, [SERVER_CONFIG_DIR, *SERVER_STATE_FILES])

    with _timed("remove heartbeat state"):
        for host in (client_host, server_host):
            _remove_paths(host, HEARTBEAT_STATE_FILES)
    with _timed("remove deploy logs (client)"):
        _remove_paths(
            client_host,
            [
                CLIENT_DEPLOY_STDOUT,
                Path(str(CLIENT_DEPLOY_STDOUT) + ".err"),
            ],
            )
    with _timed("remove deploy logs (server)"):
        _remove_paths(
            server_host,
            [
                SERVER_DEPLOY_STDOUT,
                Path(str(SERVER_DEPLOY_STDOUT) + ".err"),
            ],
            )

    server_host_ip = server_host_ipv4
    client_host_ip = client_host_ipv4
    trojan_user = "deploy-suite@example.com"
    trojan_password = "deploy-pass-123"

    client_proc = None
    server_proc = None
    try:
        with _timed("start client deploy"):
            client_proc = _start_client_deploy(
                client_host,
                remote_host=server_host_ip,
                deploy_port=DEPLOY_PORT,
                trojan_user=trojan_user,
                trojan_password=trojan_password,
                trojan_port=TROJAN_PORT,
                )
        with _timed("wait client deploy link"):
            link = _wait_for_client_link(client_host, client_proc)
        assert link.startswith("trojan://"), "xp2p client deploy did not emit trojan link"

        _set_firewall_rule(
            server_host,
            ensure="Present",
            remote_address="Any",
            port=int(DEPLOY_PORT),
            action="Allow",
            )
        _set_firewall_rule(
            server_host,
            ensure="Present",
            remote_address="Any",
            port=int(TROJAN_PORT),
            action="Allow",
            )
        with _timed("start server deploy"):
            server_proc = _start_server_deploy(
                server_host,
                listen_addr=f":{DEPLOY_PORT}",
                deploy_link=link,
                )

        with _timed("wait server deploy logs"):
            initial_server_log = _wait_for_any_log_phrase(
                server_host,
                server_proc,
                [
                    "server deploy: manifest decrypted",
                    "server deploy: starting xray-core",
                    "server deploy: starting listener",
                ],
                timeout=LOG_WAIT_TIMEOUT,
                )
            if initial_server_log == "server deploy: starting listener":
                _wait_for_log_phrase(
                    server_host,
                    server_proc,
                    "server deploy: manifest decrypted",
                    timeout=LOG_WAIT_TIMEOUT,
                    )
            if initial_server_log != "server deploy: starting xray-core":
                _wait_for_log_phrase(
                    server_host,
                    server_proc,
                    "server deploy: starting xray-core",
                    timeout=LOG_WAIT_TIMEOUT,
                    )
        with _timed("wait client deploy logs"):
            _wait_for_log_phrase(
                client_host,
                client_proc,
                "client deploy: trojan link received",
                timeout=LOG_WAIT_TIMEOUT,
                )
            _wait_for_log_phrase(
                client_host,
                client_proc,
                "client deploy: local install completed",
                timeout=LOG_WAIT_TIMEOUT,
                )
            _wait_for_any_log_phrase(
                client_host,
                client_proc,
                [
                    "client deploy: completed",
                    "client deploy: client run active",
                ],
                timeout=LOG_WAIT_TIMEOUT,
                )
        with _timed("wait server deploy completion"):
            _wait_for_any_log_phrase(
                server_host,
                server_proc,
                [
                    "server deploy: completion requested",
                    "server deploy: stopped",
                ],
                timeout=LOG_WAIT_TIMEOUT,
                )
        if client_proc:
            _stop_process(client_host, client_proc["pid"])
            client_proc = None
        if server_proc:
            _stop_process(server_host, server_proc["pid"])
            server_proc = None
        with _timed("start xp2p services"):
            xp2p_server_runner("server", "service", "start", check=True)
            xp2p_client_runner("client", "service", "start", check=True)
        with _timed("wait apply.request clear"):
            _wait_for_apply_request_clear(server_host, timeout=90.0)
            _wait_for_apply_request_clear(client_host, timeout=90.0)

        with _timed("check client internet access"):
            _assert_internet_access(client_host)

        with _timed("assert client artifacts"):
            _assert_client_install_artifacts(client_host, server_host_ip, trojan_user, trojan_password)
        with _timed("assert client state"):
            _assert_client_state(client_host, server_host_ip)
        with _timed("assert client routing"):
            _assert_client_routing(client_host, server_host_ip)

        with _timed("wait heartbeat state"):
            heartbeat = _wait_for_heartbeat_state(client_host, timeout=LOG_WAIT_TIMEOUT)
        with _timed("assert heartbeat entry"):
            _assert_heartbeat_entry(
                heartbeat,
                _expected_tag(server_host_ip),
                host=server_host_ip,
                user=trojan_user,
                client_ip=client_host_ip,
                )
    except pytest.skip.Exception:
        raise
    except Exception:
        win_env.dump_failure_state(client_host, label="client-deploy-end-to-end")
        win_env.dump_failure_state(server_host, label="server-deploy-end-to-end")
        raise
    finally:
        total = time.perf_counter() - test_start
        print(f"TIMING: test_windows_client_deploy_end_to_end total: {total:.2f}s")
        if client_proc:
            _stop_process(client_host, client_proc["pid"])
        if server_proc:
            _stop_process(server_host, server_proc["pid"])
        _stop_xp2p_processes(client_host)
        _stop_xp2p_processes(server_host)
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
        _set_firewall_rule(
            server_host,
            ensure="Absent",
            remote_address="Any",
            port=int(DEPLOY_PORT),
            action="Allow",
            )
        _set_firewall_rule(
            server_host,
            ensure="Absent",
            remote_address="Any",
            port=int(TROJAN_PORT),
            action="Allow",
            )
        for host in (client_host, server_host):
            _remove_paths(host, HEARTBEAT_STATE_FILES)


@pytest.mark.host
@pytest.mark.win
def test_windows_server_deploy_falls_back_to_self_signed_on_invalid_cert(
    client_host,
    server_host,
    client_host_ipv4,
    server_host_ipv4,
    xp2p_client_runner,
    xp2p_server_runner,
    xp2p_msi_path,
):
    _stop_xp2p_processes(client_host)
    _stop_xp2p_processes(server_host)
    xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
    xp2p_server_runner("server", "remove", "--ignore-missing")
    _remove_paths(client_host, [CLIENT_CONFIG_DIR, *CLIENT_STATE_FILES])
    _remove_paths(server_host, [SERVER_CONFIG_DIR, *SERVER_STATE_FILES])

    for host in (client_host, server_host):
        _remove_paths(host, HEARTBEAT_STATE_FILES)
    _remove_paths(
        client_host,
        [
            CLIENT_DEPLOY_STDOUT,
            Path(str(CLIENT_DEPLOY_STDOUT) + ".err"),
        ],
    )
    _remove_paths(
        server_host,
        [
            SERVER_DEPLOY_STDOUT,
            Path(str(SERVER_DEPLOY_STDOUT) + ".err"),
        ],
    )

    server_host_ip = server_host_ipv4
    trojan_user = "deploy-invalid-cert@example.com"
    trojan_password = "deploy-invalid-cert-pass"
    bad_cert = Path(r"C:\Windows\Temp\xp2p-invalid-cert.pem")
    bad_key = Path(r"C:\Windows\Temp\xp2p-invalid-key.pem")

    client_proc = None
    server_proc = None
    try:
        _remove_remote_path(server_host, bad_cert)
        _remove_remote_path(server_host, bad_key)

        client_proc = _start_client_deploy(
            client_host,
            remote_host=server_host_ip,
            deploy_port=DEPLOY_PORT,
            trojan_user=trojan_user,
            trojan_password=trojan_password,
            trojan_port=TROJAN_PORT,
            )
        link = _wait_for_client_link(client_host, client_proc)

        _set_firewall_rule(
            server_host,
            ensure="Present",
            remote_address=client_host_ipv4,
            port=int(DEPLOY_PORT),
            action="Allow",
            )
        server_proc = _start_server_deploy_with_args(
            server_host,
            listen_addr=f":{DEPLOY_PORT}",
            deploy_link=link,
            env_overrides={
                "XP2P_SERVER_CERTIFICATE": str(bad_cert),
                "XP2P_SERVER_KEY": str(bad_key),
            },
            )

        initial_server_log = _wait_for_any_log_phrase(
            server_host,
            server_proc,
            [
                "server deploy: manifest decrypted",
                "server deploy: starting xray-core",
            ],
            timeout=LOG_WAIT_TIMEOUT,
            )
        _wait_for_log_phrase(
            server_host,
            server_proc,
            "server deploy: certificate validation failed, using self-signed",
            timeout=LOG_WAIT_TIMEOUT,
            )
        if initial_server_log != "server deploy: starting xray-core":
            _wait_for_log_phrase(
                server_host,
                server_proc,
                "server deploy: starting xray-core",
                timeout=LOG_WAIT_TIMEOUT,
                )
        _wait_for_log_phrase(
            client_host,
            client_proc,
            "client deploy: local install completed",
            timeout=LOG_WAIT_TIMEOUT,
            )

        assert _remote_path_exists(server_host, SERVER_CERT_DEST), (
            f"Expected server cert at {SERVER_CERT_DEST}"
        )
        assert _remote_path_exists(server_host, SERVER_KEY_DEST), (
            f"Expected server key at {SERVER_KEY_DEST}"
        )
        inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        trojan = _find_trojan_inbound(inbounds)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates after deploy fallback"
        primary = certificates[0]
        expected_cert_paths = {
            _normalize_windows_path(str(SERVER_CERT_DEST)),
            _normalize_windows_path(str(win_env.pending_candidate(SERVER_CERT_DEST))),
            _normalize_windows_path(str(bad_cert)),
        }
        expected_key_paths = {
            _normalize_windows_path(str(SERVER_KEY_DEST)),
            _normalize_windows_path(str(win_env.pending_candidate(SERVER_KEY_DEST))),
            _normalize_windows_path(str(bad_key)),
        }
        assert _normalize_windows_path(primary.get("certificateFile")) in expected_cert_paths
        assert _normalize_windows_path(primary.get("keyFile")) in expected_key_paths
    finally:
        if client_proc:
            _stop_process(client_host, client_proc["pid"])
        if server_proc:
            _stop_process(server_host, server_proc["pid"])
        _stop_xp2p_processes(client_host)
        _stop_xp2p_processes(server_host)
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
        xp2p_server_runner("server", "remove", "--ignore-missing")
        _set_firewall_rule(
            server_host,
            ensure="Absent",
            remote_address=client_host_ipv4,
            port=int(DEPLOY_PORT),
            action="Allow",
            )
        for host in (client_host, server_host):
            _remove_paths(host, HEARTBEAT_STATE_FILES)


def _set_firewall_rule(
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

def _start_client_deploy(
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
    pid = _extract_marker(result.stdout, "PID=")
    stdout_path = _extract_marker(result.stdout, "STDOUT=")
    stderr_path = _extract_marker(result.stdout, "STDERR=")
    if not pid or not stdout_path or not stderr_path:
        pytest.fail(
            "xp2p client deploy script did not emit expected markers.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return {"pid": int(pid), "stdout": Path(stdout_path), "stderr": Path(stderr_path)}


def _start_server_deploy(host, *, listen_addr: str, deploy_link: str) -> dict[str, str | int]:
    return _start_server_deploy_with_args(host, listen_addr=listen_addr, deploy_link=deploy_link)


def _start_server_deploy_with_args(
    host,
    *,
    listen_addr: str,
    deploy_link: str,
    additional_args: list[str] | None = None,
    env_overrides: dict[str, str] | None = None,
) -> dict[str, str | int]:
    parameters: dict[str, object] = {
        "Xp2pPath": str(win_env.XP2P_EXE),
        "LogPath": str(SERVER_DEPLOY_STDOUT),
        "ListenAddress": listen_addr,
        "DeployLink": deploy_link,
    }
    if additional_args:
        parameters["AdditionalArgsBase64"] = _encode_args_payload(additional_args)
    if env_overrides:
        parameters["EnvOverridesBase64"] = _encode_env_payload(env_overrides)
    result = win_env.run_guest_script(
        host,
        "scripts/start_xp2p_server_deploy.ps1",
        **parameters,
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


@contextmanager
def _timed(label: str):
    start = time.perf_counter()
    print(f"TIMING: start {label}")
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: {label}: {elapsed:.2f}s")


def _encode_args_payload(args: list[str]) -> str:
    raw = json.dumps([str(arg) for arg in args])
    return base64.b64encode(raw.encode("utf-8")).decode("ascii")


def _encode_env_payload(env_overrides: dict[str, str]) -> str:
    raw = json.dumps({str(key): str(value) for key, value in env_overrides.items()})
    return base64.b64encode(raw.encode("utf-8")).decode("ascii")


def _stop_process(host, pid: int) -> None:
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$targetPid = {pid}
if ($targetPid -le 0) {{
    exit 0
}}
$proc = Get-Process -Id $targetPid -ErrorAction SilentlyContinue
if ($proc) {{
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}}
exit 0
"""
    result = win_env.run_powershell(host, script, label="stop_xp2p_processes")
    if result.rc != 0:
        combined = f"{result.stdout}\n{result.stderr}"
        if "SetConsoleWindowTitle" in combined or "No process is on the other end of the pipe" in combined:
            return
        pytest.fail(
            f"Failed to stop process {pid}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _stop_xp2p_processes(host) -> None:
    script = """
$ErrorActionPreference = 'Stop'
Get-Process -Name xp2p,xray -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
exit 0
"""
    result = win_env.run_powershell(host, script, label="stop_listening_ports")
    if result.rc != 0:
        pytest.fail(
            "Failed to stop xp2p processes.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _stop_listening_ports(host, ports: list[int]) -> None:
    ports_json = json.dumps([int(port) for port in ports])
    payload = base64.b64encode(ports_json.encode("utf-8")).decode("ascii")
    script = f"""
$ErrorActionPreference = 'Stop'
$payload = {win_env.ps_quote(payload)}
$decoded = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($payload))
$ports = $decoded | ConvertFrom-Json
if (-not ($ports -is [System.Collections.IEnumerable])) {{
    exit 0
}}
$targets = @{{}}
foreach ($port in $ports) {{
    $value = 0
    if ([int]::TryParse([string]$port, [ref]$value)) {{
        if ($value -gt 0) {{
            $targets[$value] = $true
        }}
    }}
}}
if ($targets.Count -eq 0) {{
    exit 0
}}
$netTcpCmd = Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue
$netUdpCmd = Get-Command Get-NetUDPEndpoint -ErrorAction SilentlyContinue
if ($netTcpCmd -or $netUdpCmd) {{
    foreach ($port in $targets.Keys) {{
        if ($netTcpCmd) {{
            $listeners = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
            foreach ($listener in $listeners) {{
                if ($listener.OwningProcess -gt 0) {{
                    try {{
                        Stop-Process -Id $listener.OwningProcess -Force -ErrorAction SilentlyContinue
                    }} catch {{ }}
                }}
            }}
        }}
        if ($netUdpCmd) {{
            $endpoints = Get-NetUDPEndpoint -LocalPort $port -ErrorAction SilentlyContinue
            foreach ($endpoint in $endpoints) {{
                if ($endpoint.OwningProcess -gt 0) {{
                    try {{
                        Stop-Process -Id $endpoint.OwningProcess -Force -ErrorAction SilentlyContinue
                    }} catch {{ }}
                }}
            }}
        }}
    }}
    exit 0
}}
$lines = netstat -ano -p tcp | Select-String -Pattern "LISTENING"
foreach ($match in $lines) {{
    $line = $match.Line
    if (-not $line) {{
        continue
    }}
    $parts = $line -split "\\s+"
    if ($parts.Length -lt 5) {{
        continue
    }}
    $local = $parts[1]
    $pid = $parts[-1]
    if ($local -match ":(\\d+)$") {{
        $port = [int]$Matches[1]
        if ($targets.ContainsKey($port)) {{
            try {{
                Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
            }} catch {{ }}
        }}
    }}
}}
if ($targets.Count -gt 0) {{
    $udpLines = netstat -ano -p udp
    foreach ($line in $udpLines) {{
        if (-not $line) {{
            continue
        }}
        $parts = $line -split "\\s+"
        if ($parts.Length -lt 4) {{
            continue
        }}
        $local = $parts[1]
        $pid = $parts[-1]
        if ($local -match ":(\\d+)$") {{
            $port = [int]$Matches[1]
            if ($targets.ContainsKey($port)) {{
                try {{
                    Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
                }} catch {{ }}
            }}
        }}
    }}
}}
exit 0
"""
    result = win_env.run_powershell(host, script, label="check_internet_access")
    if result.rc != 0:
        pytest.fail(
            "Failed to stop listening ports.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _assert_client_install_artifacts(host, server_ip: str, user: str, password: str) -> None:
    assert _remote_path_exists(host, CLIENT_CONFIG_OUTBOUNDS), "client config directory missing after deploy"
    outbounds = _read_remote_json(host, CLIENT_CONFIG_OUTBOUNDS)
    _assert_outbound_entry(
        outbounds,
        server_ip,
        password,
        user,
        server_ip,
        pinned_peer_sha256="",
        verify_peer_name=server_ip,
    )


def _assert_client_state(host, server_ip: str) -> None:
    state = win_env.read_toml(host, CLIENT_CONFIG_FILE).get("client") or {}
    recorded_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
    assert recorded_hosts == {server_ip}, f"Unexpected endpoint entries recorded: {recorded_hosts}"


def _assert_client_routing(host, server_ip: str) -> None:
    routing = _read_remote_json(host, CLIENT_ROUTING_JSON)
    _assert_routing_rule(routing, server_ip)


def _assert_internet_access(host) -> None:
    script = """
$ErrorActionPreference = 'Stop'
$dnsName = "example.com"
$tcpHost = "1.1.1.1"
$tcpPort = 443
try {
    Resolve-DnsName -Name $dnsName -ErrorAction Stop | Out-Null
} catch {
    Write-Error "Internet check failed: DNS lookup for $dnsName"
    exit 1
}
try {
    $tcpOk = Test-NetConnection -ComputerName $tcpHost -Port $tcpPort -InformationLevel Quiet
} catch {
    $tcpOk = $false
}
if (-not $tcpOk) {
    Write-Error "Internet check failed: TCP connect to ${tcpHost}:${tcpPort}"
    exit 1
}
exit 0
"""
    result = win_env.run_powershell(host, script, label="read_optional_text")
    if result.rc != 0:
        pytest.fail(
            "Client internet check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _wait_for_client_link(host, proc_info: dict[str, str | int]) -> str:
    def _extract_link(text: str) -> str | None:
        for line in text.splitlines():
            if "client deploy: link generated" not in line:
                continue
            if "link:" not in line:
                continue
            return line.split("link:", 1)[1].strip()
        return None

    link = _wait_for_log_value(
        host,
        proc_info,
        extractor=_extract_link,
        description="xp2p client deploy link",
        timeout=LOG_WAIT_TIMEOUT,
    )
    if not link:
        pytest.fail("xp2p client deploy log did not include a deploy link")
    return link


def _wait_for_log_phrase(host, proc_info: dict[str, str | int], phrase: str, *, timeout: int) -> None:
    expected_variants = (phrase, f"xp2p: {phrase}")

    def _matcher(text: str) -> bool | None:
        for variant in expected_variants:
            if variant in text:
                return True
        return None

    _wait_for_log_value(host, proc_info, extractor=_matcher, description=f"'{phrase}'", timeout=timeout)


def _wait_for_any_log_phrase(
    host,
    proc_info: dict[str, str | int],
    phrases: list[str],
    *,
    timeout: int,
) -> str:
    expected_variants = [(phrase, f"xp2p: {phrase}") for phrase in phrases]

    def _matcher(text: str) -> str | None:
        for phrase, prefixed in expected_variants:
            if phrase in text or prefixed in text:
                return phrase
        return None

    return _wait_for_log_value(
        host,
        proc_info,
        extractor=_matcher,
        description=f"any of {phrases}",
        timeout=timeout,
    )


def _wait_for_apply_request_clear(host, *, timeout: float = 60.0) -> None:
    apply_path = win_env.CONFIG_ROOT / win_env.APPLY_DIR_NAME / "apply.request"
    deadline = time.time() + timeout
    while time.time() < deadline:
        if not win_env.path_exists(host, apply_path):
            return
        time.sleep(1.0)
    pytest.fail(f"apply.request did not clear after {timeout} seconds.")


def _wait_for_log_value(
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
        stdout_text = _read_optional_text(host, proc_info["stdout"])
        stderr_text = _read_optional_text(host, proc_info["stderr"])
        combined = "\n".join(filter(None, [stdout_text, stderr_text]))
        if combined:
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
        "Timed out waiting for "
        f"{description}.\nFailure dump: {dump_path}\n"
        f"STDOUT tail:\n{stdout_tail}\nSTDERR tail:\n{stderr_tail}"
    )


def _wait_for_ping_ok_or_server_failure(
    client_host,
    client_proc: dict[str, str | int],
    server_host,
    server_proc: dict[str, str | int],
    *,
    timeout: int,
) -> None:
    deadline = time.time() + timeout
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        stdout_text = _read_optional_text(client_host, client_proc["stdout"])
        stderr_text = _read_optional_text(client_host, client_proc["stderr"])
        combined = "\n".join(filter(None, [stdout_text, stderr_text]))
        if "client deploy: ping ok" in combined or "xp2p: client deploy: ping ok" in combined:
            return
        last_stdout = stdout_text or last_stdout
        last_stderr = stderr_text or last_stderr
        server_logs = _read_combined_logs(server_host, server_proc)
        if "server deploy: xray-core start failed" in server_logs:
            dump_path = win_env.dump_failure_state(server_host, label="server-deploy-xray-failed")
            pytest.fail(
                "Server deploy xray-core failed while waiting for client ping.\n"
                f"Server logs:\n{server_logs}\n\nFailure dump: {dump_path}"
        )
        if "server deploy: stopped" in server_logs:
            if not _service_running(server_host, "xp2p-server"):
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


def _read_combined_logs(host, proc_info: dict[str, str | int]) -> str:
    stdout_text = _read_optional_text(host, proc_info["stdout"])
    stderr_text = _read_optional_text(host, proc_info["stderr"])
    return "\n".join(filter(None, [stdout_text, stderr_text]))


def _service_running(host, service_name: str) -> bool:
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


def _read_optional_text(host, path_value) -> str:
    path = Path(path_value)
    script = f"""
$ErrorActionPreference = 'Stop'
$path = {win_env.ps_quote(str(path))}
if (-not (Test-Path $path)) {{
    exit 3
}}
Get-Content -Path $path -Raw
exit 0
"""
    result = win_env.run_powershell(host, script, label="read_optional_text")
    if result.rc == 0:
        return result.stdout or ""
    if result.rc == 3:
        return ""
    pytest.fail(
        f"Failed to read remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def _read_remote_json(client_host, path: Path) -> dict:
    resolved = win_env.resolve_config_path(client_host, path)
    script = f"""
$ErrorActionPreference = 'Stop'
$path = {win_env.ps_quote(str(resolved))}
if (-not (Test-Path $path)) {{
    exit 3
}}
Get-Content -Path $path -Raw
exit 0
"""
    result = win_env.run_powershell(client_host, script, label="read_remote_json")
    if result.rc != 0:
        pytest.fail(
            f"Failed to read remote JSON {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{result.stdout}")


def _read_first_existing_json(host, paths: list[Path]) -> dict:
    for path in paths:
        if _remote_path_exists(host, path):
            return _read_remote_json(host, path)
    raise AssertionError(f"None of the state files exist: {paths}")


def _remote_path_exists(client_host, path: Path) -> bool:
    resolved = win_env.resolve_config_path(client_host, path)
    target = win_env.ps_quote(str(resolved))
    result = win_env.run_powershell(
        client_host,
        f"if (Test-Path {target}) {{ exit 0 }} else {{ exit 3 }}",
        label="remote_path_exists",
    )
    if result.rc == 0:
        return True
    if result.rc == 3:
        return False
    if not (result.stdout or result.stderr):
        return False
    pytest.fail(
        f"Failed to check remote path {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def _remove_remote_path(client_host, path: Path) -> None:
    _remove_paths(client_host, [path])


def _remove_paths(client_host, paths: list[Path]) -> None:
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
    result = win_env.run_powershell(client_host, script, label="remove_paths")
    if result.rc != 0:
        pytest.fail(
            "Failed to remove remote paths.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _expected_tag(host: str) -> str:
    cleaned = host.strip().lower()
    result = []
    last_dash = False
    for char in cleaned:
        if char.isalnum():
            result.append(char)
            last_dash = False
            continue
        if char == "-":
            result.append(char)
            last_dash = False
            continue
        if not last_dash:
            result.append("-")
            last_dash = True
    sanitized = "".join(result).strip("-")
    if not sanitized:
        sanitized = "endpoint"
    return f"proxy-{sanitized}"


def _find_trojan_inbound(data: dict) -> dict:
    for inbound in data.get("inbounds", []):
        if inbound.get("protocol") == "trojan":
            return inbound
    raise AssertionError("Expected trojan inbound in server configuration")


def _normalize_windows_path(value: str | None) -> str:
    return (value or "").replace("\\", "/")


def _assert_outbound_entry(
    data: dict,
    host: str,
    password: str,
    email: str,
    server_name: str,
    allow_insecure: bool = False,
    pinned_peer_sha256: str | None = None,
    verify_peer_name: str | None = None,
) -> None:
    tag = _expected_tag(host)
    outbound = _find_outbound(data, tag)
    server = outbound["settings"]["servers"][0]
    assert server["address"] == host
    assert server["password"] == password
    assert server["email"] == email
    tls_settings = outbound["streamSettings"]["tlsSettings"]
    assert tls_settings["serverName"] == server_name
    if pinned_peer_sha256 is not None:
        actual_pin = tls_settings.get("pinnedPeerCertSha256")
        if pinned_peer_sha256:
            assert actual_pin == pinned_peer_sha256
        else:
            assert actual_pin, "Expected pinnedPeerCertSha256 to be set"
        if verify_peer_name:
            assert tls_settings.get("verifyPeerCertByName") == verify_peer_name
        assert "allowInsecure" not in tls_settings or not tls_settings.get("allowInsecure")
    else:
        assert bool(tls_settings.get("allowInsecure")) is bool(allow_insecure)


def _find_outbound(data: dict, tag: str) -> dict:
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            return outbound
    raise AssertionError(f"Expected outbound with tag {tag} to exist")


def _assert_routing_rule(data: dict, host: str) -> None:
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") == "direct" and host in rule.get("ip", []):
            return
    raise AssertionError(f"Expected routing rule for {host} -> direct")


def _wait_for_heartbeat_state(host, *, timeout: int) -> dict:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        for path in HEARTBEAT_STATE_FILES:
            if _remote_path_exists(host, path):
                try:
                    return _read_remote_json(host, path)
                except Exception as exc:  # noqa: BLE001
                    last_error = exc
        time.sleep(1)
    if last_error:
        raise AssertionError(f"Failed to read heartbeat state: {last_error}") from last_error
    raise AssertionError("Heartbeat state file not found on client host")


def _assert_heartbeat_entry(
    state: dict,
    tag: str,
    *,
    host: str | None = None,
    user: str | None = None,
    client_ip: str | None = None,
) -> None:
    entries = state.get("entries")
    if not isinstance(entries, dict):
        raise AssertionError("Heartbeat state is missing entries map")
    normalized = (tag or "").strip().lower()
    if not normalized:
        raise AssertionError("Heartbeat tag to look up is empty")
    for entry in entries.values():
        entry_tag = (entry.get("tag") or "").strip()
        if entry_tag.lower() != normalized:
            continue
        if host is not None:
            recorded_host = (entry.get("host") or "").strip()
            if recorded_host != host.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} host mismatch (expected {host}, got {recorded_host})"
        )
        if user is not None:
            recorded_user = (entry.get("user") or "").strip()
            if recorded_user != user.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} user mismatch (expected {user}, got {recorded_user})"
        )
        if client_ip is not None:
            recorded_ip = (entry.get("client_ip") or "").strip()
            if recorded_ip != client_ip.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} client IP mismatch (expected {client_ip}, got {recorded_ip})"
        )
        return
    raise AssertionError(f"Heartbeat entry for tag {tag} not found in state")


def _detect_host_ipv4(host) -> str:
    script = """
$ErrorActionPreference = 'Stop'
$addresses = Get-NetIPAddress -AddressFamily IPv4 -PrefixOrigin (@('Dhcp', 'Manual')) |
    Where-Object { $_.IPAddress -ne '127.0.0.1' } |
    Select-Object -ExpandProperty IPAddress
if (-not $addresses) {
    exit 3
}
$addresses
"""
    result = win_env.run_powershell(host, script, label="stop_process")
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


def _extract_marker(output: str | None, marker: str) -> str | None:
    for raw in (output or "").splitlines():
        line = raw.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None
