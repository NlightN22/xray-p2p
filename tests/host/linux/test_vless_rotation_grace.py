from __future__ import annotations

import base64
import json
import shlex
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as runtime
from tests.host.linux import env as linux_env


CONTROL_PORT = "62022"
SERVER_HOST = "vless-rotation.example.com"
SERVER_PORT = "62311"
USER = "vless-rotation@example.com"
PREVIOUS = "550e8400-e29b-41d4-a716-446655440010"
PROBE_ROOT = "/tmp/xp2p-vless-rotation-probe"
SOCKS_PORT = 51191

pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.serial]


def test_vless_previous_uuid_works_until_ack(client_host, server_host):
    runner = runtime.xp2p_runner(server_host)
    server_ip = _detect_host_ipv4(server_host)
    try:
        runner(
            "server", "install", "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME, "--host", SERVER_HOST,
            "--port", SERVER_PORT, "--force", check=True,
        )
        runner("server", "profile", "vless-tls-vision", check=True)
        runner(
            "server", "user", "add", "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME, "--id", USER,
            "--password", PREVIOUS, "--host", SERVER_HOST, check=True,
        )
        server_pin = _server_cert_pin(server_host)
        runtime.start_service(server_host, runner, "server")
        runner("server", "user", "rotate", USER, check=True)

        rotated = _wait_for_rotation(server_host)
        active = rotated["active_credential"]
        assert active != PREVIOUS
        _wait_for_live_credentials(server_host, {active, PREVIOUS})
        _probe_vless(client_host, server_ip, server_pin, PREVIOUS, should_succeed=True)

        acknowledged = linux_env.run_guest_script(
            server_host, "scripts/linux/check_credential_rotation.sh",
            "127.0.0.1", CONTROL_PORT, USER, PREVIOUS, timeout=60,
        )
        assert acknowledged.rc == 0, acknowledged.stderr
        _wait_for_live_credentials(server_host, {active})

        _probe_vless(client_host, server_ip, server_pin, PREVIOUS, should_succeed=False)
        _probe_vless(client_host, server_ip, server_pin, active, should_succeed=True)
    finally:
        _stop_probe(client_host)
        runtime.stop_service(runner, "server")


def _probe_vless(host, server_ip: str, server_pin: str, credential: str, *, should_succeed: bool) -> None:
    _stop_probe(host)
    config = {
        "inbounds": [{
            "listen": "127.0.0.1", "port": SOCKS_PORT, "protocol": "socks",
            "settings": {"auth": "noauth", "udp": False},
        }],
        "outbounds": [{
            "protocol": "vless",
            "settings": {"vnext": [{"address": server_ip, "port": int(SERVER_PORT), "users": [{
                "id": credential, "encryption": "none", "flow": "xtls-rprx-vision",
            }]}]},
            "streamSettings": {"network": "tcp", "security": "tls", "tlsSettings": {
                "serverName": SERVER_HOST,
                "pinnedPeerCertSha256": server_pin,
                "verifyPeerCertByName": SERVER_HOST,
            }},
        }],
    }
    encoded = base64.b64encode(json.dumps(config).encode()).decode()
    script = (
        f"install -d -m 0700 {PROBE_ROOT}; echo {shlex.quote(encoded)} | base64 -d > {PROBE_ROOT}/config.json; "
        f"nohup /etc/xp2p/bin/xray run -config {PROBE_ROOT}/config.json >{PROBE_ROOT}/xray.log 2>&1 & "
        f"echo $! > {PROBE_ROOT}/xray.pid"
    )
    started = host.run(f"sudo -n /bin/sh -c {shlex.quote(script)}")
    assert started.rc == 0, started.stderr
    deadline = time.time() + 30
    last = None
    while time.time() < deadline:
        last = host.run(
            f"curl --fail --silent --max-time 5 --socks5-hostname 127.0.0.1:{SOCKS_PORT} "
            "--output /dev/null https://example.com/"
        )
        if (last.rc == 0) == should_succeed:
            return
        time.sleep(1)
    expectation = "succeed" if should_succeed else "fail"
    diagnostics = host.run(
        f"sudo -n /bin/sh -c 'cat {PROBE_ROOT}/xray.log 2>/dev/null || true; "
        f"echo ---config---; cat {PROBE_ROOT}/config.json 2>/dev/null || true'"
    )
    raise AssertionError(
        f"VLESS probe did not {expectation}; last exit={last.rc if last else 'none'}\n"
        f"{diagnostics.stdout}\n{diagnostics.stderr}"
    )


def _stop_probe(host) -> None:
    host.run(
        f"sudo -n /bin/sh -c 'test ! -f {PROBE_ROOT}/xray.pid || "
        f"kill $(cat {PROBE_ROOT}/xray.pid) 2>/dev/null || true; rm -rf {PROBE_ROOT}'"
    )


def _wait_for_rotation(host) -> dict:
    deadline = time.time() + 30
    while time.time() < deadline:
        desired = helpers.read_pending_server_config(host)
        for user in desired.get("users") or []:
            if user.get("user_label") == USER and user.get("previous_credential_for_rotation") == PREVIOUS:
                return user
        time.sleep(1)
    raise AssertionError("VLESS rotation state was not persisted")


def _wait_for_live_credentials(host, expected: set[str]) -> None:
    deadline = time.time() + 30
    while time.time() < deadline:
        live = runtime.wait_for_live_xray(host, "server")
        actual = {
            str(user.get("id"))
            for inbound in live.get("inbounds") or [] if inbound.get("protocol") == "vless"
            for user in inbound.get("settings", {}).get("clients") or []
            if user.get("email") in {USER, f"{USER}.previous"}
        }
        if actual == expected:
            return
        time.sleep(1)
    raise AssertionError(f"VLESS Live credentials did not become {sorted(expected)}")


def _detect_host_ipv4(host) -> str:
    result = host.run("ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1")
    assert result.rc == 0, result.stderr
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    assert addresses, "server has no global IPv4 address"
    return next((address for address in addresses if not address.startswith("10.0.2.")), addresses[0])


def _server_cert_pin(host) -> str:
    cert_path = helpers.INSTALL_ROOT / "tls" / "server" / "cert.pem"
    result = host.run(
        f"sudo -n openssl x509 -in {cert_path.as_posix()} -outform DER | sha256sum | cut -d' ' -f1"
    )
    assert result.rc == 0, result.stderr
    pin = (result.stdout or "").strip()
    assert len(pin) == 64, f"invalid server certificate pin: {pin!r}"
    return pin
