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
    with _timed("xp2p client remove"):
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
    with _timed("xp2p server remove"):
        xp2p_server_runner("server", "remove", "--ignore-missing")
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
            _wait_for_log_phrase(
                server_host,
                server_proc,
                "server deploy: manifest decrypted",
                timeout=LOG_WAIT_TIMEOUT,
            )
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
            _wait_for_log_phrase(
                client_host,
                client_proc,
                "client deploy: ping ok",
                timeout=LOG_WAIT_TIMEOUT,
            )
            _wait_for_log_phrase(
                client_host,
                client_proc,
                "client deploy: client run active",
                timeout=LOG_WAIT_TIMEOUT,
            )

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
        xp2p_server_runner("server", "remove", "--ignore-missing")
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
        _write_server_config(server_host, certificate=bad_cert, key=bad_key)
        server_proc = _start_server_deploy(
            server_host,
            listen_addr=f":{DEPLOY_PORT}",
            deploy_link=link,
        )

        _wait_for_log_phrase(
            server_host,
            server_proc,
            "server deploy: manifest decrypted",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            server_host,
            server_proc,
            "server deploy: certificate validation failed, using self-signed",
            timeout=LOG_WAIT_TIMEOUT,
        )
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
        assert tls_settings.get("allowInsecure") is True
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates after deploy fallback"
        primary = certificates[0]
        assert _normalize_windows_path(primary.get("certificateFile")) == str(SERVER_CERT_DEST).replace("\\", "/")
        assert _normalize_windows_path(primary.get("keyFile")) == str(SERVER_KEY_DEST).replace("\\", "/")
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
    result = win_env.run_guest_script(
        server_host,
        "scripts/configure_firewall_rule.ps1",
        Name=rule_name,
        RemoteAddress=remote_address,
        LocalPort=str(port),
        Ensure=ensure,
        Protocol="TCP",
        Action=action,
    )
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
) -> dict[str, str | int]:
    parameters: dict[str, object] = {
        "Xp2pPath": str(win_env.XP2P_EXE),
        "LogPath": str(SERVER_DEPLOY_STDOUT),
        "ListenAddress": listen_addr,
        "DeployLink": deploy_link,
    }
    if additional_args:
        parameters["AdditionalArgsBase64"] = _encode_args_payload(additional_args)
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
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: {label}: {elapsed:.2f}s")


def _encode_args_payload(args: list[str]) -> str:
    raw = json.dumps([str(arg) for arg in args])
    return base64.b64encode(raw.encode("utf-8")).decode("ascii")


def _stop_process(host, pid: int) -> None:
    result = win_env.run_guest_script(
        host,
        "scripts/stop_process.ps1",
        ProcessId=str(pid),
    )
    if result.rc != 0:
        exists = win_env.run_guest_script(
            host,
            "scripts/process_exists.ps1",
            ProcessId=str(pid),
        )
        if exists.rc == 3:
            return
        pytest.fail(
            f"Failed to stop process {pid}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _stop_xp2p_processes(host) -> None:
    result = win_env.run_guest_script(
        host,
        "scripts/kill_xp2p_processes.ps1",
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to stop xp2p processes.\n"
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
        allow_insecure=True,
    )


def _assert_client_state(host, server_ip: str) -> None:
    state = win_env.read_toml(host, CLIENT_CONFIG_FILE).get("client") or {}
    recorded_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
    assert recorded_hosts == {server_ip}, f"Unexpected endpoint entries recorded: {recorded_hosts}"


def _assert_client_routing(host, server_ip: str) -> None:
    routing = _read_remote_json(host, CLIENT_ROUTING_JSON)
    _assert_routing_rule(routing, server_ip)


def _assert_internet_access(host) -> None:
    result = win_env.run_guest_script(
        host,
        "scripts/check_internet_win.ps1",
    )
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


def _wait_for_log_value(
    host,
    proc_info: dict[str, str | int],
    *,
    extractor,
    description: str,
    timeout: int,
):
    deadline = time.time() + timeout
    last_text = ""
    while time.time() < deadline:
        text = _read_combined_logs(host, proc_info)
        if text:
            value = extractor(text)
            if value:
                return value
            last_text = text
        time.sleep(1)
    tail = "\n".join((last_text or "").splitlines()[-30:])
    pytest.fail(f"Timed out waiting for {description}. Recent log tail:\n{tail}")


def _read_combined_logs(host, proc_info: dict[str, str | int]) -> str:
    stdout_text = _read_optional_text(host, proc_info["stdout"])
    stderr_text = _read_optional_text(host, proc_info["stderr"])
    return "\n".join(filter(None, [stdout_text, stderr_text]))


def _read_optional_text(host, path_value) -> str:
    path = Path(path_value)
    result = win_env.run_guest_script(
        host,
        "scripts/read_file.ps1",
        Path=str(path),
    )
    if result.rc == 0:
        return result.stdout or ""
    if result.rc == 3:
        return ""
    pytest.fail(
        f"Failed to read remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def _read_remote_json(client_host, path: Path) -> dict:
    result = win_env.run_guest_script(
        client_host,
        "scripts/read_file.ps1",
        Path=str(path),
    )
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
    result = win_env.run_guest_script(
        client_host,
        "scripts/path_exists.ps1",
        force_stage=True,
        TargetPath=str(path),
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
    payload = base64.b64encode(
        json.dumps([str(path) for path in paths]).encode("utf-8")
    ).decode("ascii")
    result = win_env.run_guest_script(
        client_host,
        "scripts/remove_paths.ps1",
        PathsBase64=payload,
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to remove remote paths.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _toml_escape_path(value: Path) -> str:
    return str(value).replace("\\", "\\\\")


def _write_server_config(host, *, certificate: Path | None = None, key: Path | None = None) -> None:
    lines = ["[server]"]
    if certificate is not None:
        lines.append(f'certificate = "{_toml_escape_path(certificate)}"')
    if key is not None:
        lines.append(f'key = "{_toml_escape_path(key)}"')
    win_env.write_text(host, SERVER_CONFIG_FILE, "\n".join(lines) + "\n")


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
) -> None:
    tag = _expected_tag(host)
    outbound = _find_outbound(data, tag)
    server = outbound["settings"]["servers"][0]
    assert server["address"] == host
    assert server["password"] == password
    assert server["email"] == email
    tls_settings = outbound["streamSettings"]["tlsSettings"]
    assert tls_settings["serverName"] == server_name
    assert bool(tls_settings.get("allowInsecure")) is bool(allow_insecure)


def _find_outbound(data: dict, tag: str) -> dict:
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            return outbound
    raise AssertionError(f"Expected outbound with tag {tag} to exist")


def _assert_routing_rule(data: dict, host: str) -> None:
    tag = _expected_tag(host)
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") == tag and host in rule.get("ip", []):
            return
    raise AssertionError(f"Expected routing rule for {host} -> {tag}")


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
    result = win_env.run_guest_script(
        host,
        "scripts/get_ipv4_addresses.ps1",
    )
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
