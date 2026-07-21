from __future__ import annotations

import base64
from contextlib import contextmanager
import hashlib
import json
from pathlib import PurePosixPath
import shlex
import time


SERVER_IP = "10.62.10.12"
TLS_NAME = "xp2p-integration.local"
TROJAN_PORT = 58443
VLESS_PORT = 58444
LOCAL_HTTP_PORT = 18080
SOCKS_ADDRESS = "127.0.0.1:51180"
EXTERNAL_IP_URL = "https://api.ipify.org"
TROJAN_PASSWORD = "bare-trojan-password-!_2026"
VLESS_UUID = "550e8400-e29b-41d4-a716-446655440000"
ROOT = PurePosixPath("/tmp/xp2p-bare-xray")
CERT = PurePosixPath("/srv/xray-p2p/tests/fixtures/tls/integration-cert.pem")
KEY = PurePosixPath("/srv/xray-p2p/tests/fixtures/tls/integration-key.pem")
XRAY = PurePosixPath("/etc/xp2p/bin/xray")
def certificate_pin(host) -> str:
    command = (
        f"openssl x509 -in {CERT} -outform DER 2>/dev/null | sha256sum | cut -d' ' -f1"
    )
    result = host.run(command)
    assert result.rc == 0, result.stderr
    return result.stdout.strip().lower()
def connection_link(protocol: str, pin: str, *, verify_name: str = TLS_NAME) -> str:
    common = f"security=tls&type=tcp&sni={TLS_NAME}&xp2p_verify_name={verify_name}"
    if pin:
        common += f"&xp2p_pin_sha256={pin}"
    if protocol == "trojan":
        return f"trojan://{TROJAN_PASSWORD}@{TLS_NAME}:{TROJAN_PORT}?{common}#bare-trojan"
    if protocol == "vless":
        return (
            f"vless://{VLESS_UUID}@{TLS_NAME}:{VLESS_PORT}?{common}"
            "&flow=xtls-rprx-vision&encryption=none#bare-vless"
        )
    raise ValueError(f"Unsupported fixture protocol: {protocol}")
def _server_config() -> dict:
    def inbound(protocol: str, port: int, credential: str) -> dict:
        user = {"email": f"bare-{protocol}", "level": 0}
        if protocol == "trojan":
            user["password"] = credential
        else:
            user.update({"id": credential, "flow": "xtls-rprx-vision"})
        return {
            "tag": f"bare-{protocol}-in",
            "listen": "0.0.0.0",
            "port": port,
            "protocol": protocol,
            "settings": {"clients": [user], **({"decryption": "none"} if protocol == "vless" else {})},
            "streamSettings": {
                "network": "tcp",
                "security": "tls",
                "tlsSettings": {
                    "serverName": TLS_NAME,
                    "certificates": [{"certificateFile": str(CERT), "keyFile": str(KEY)}],
                },
            },
        }
    return {
        "log": {"loglevel": "warning", "access": str(ROOT / "access.log"), "error": str(ROOT / "error.log")},
        "inbounds": [
            inbound("trojan", TROJAN_PORT, TROJAN_PASSWORD),
            inbound("vless", VLESS_PORT, VLESS_UUID),
        ],
        "outbounds": [{"tag": "direct", "protocol": "freedom", "settings": {}}],
        "routing": {
            "domainStrategy": "IPIfNonMatch",
            "rules": [
                {"type": "field", "ip": ["127.0.0.1/32"], "outboundTag": "direct"},
                {"type": "field", "domain": ["full:api.ipify.org"], "outboundTag": "direct"},
            ],
        },
    }
def _encoded(value: object) -> str:
    raw = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return base64.b64encode(raw).decode()
def _wait_ready(host) -> None:
    deadline = time.time() + 20
    probe = (
        f"kill -0 $(cat {ROOT}/xray.pid) 2>/dev/null && "
        f"ss -ltn | grep -q ':{TROJAN_PORT} ' && ss -ltn | grep -q ':{VLESS_PORT} ' && "
        f"ss -ltn | grep -q ':{LOCAL_HTTP_PORT} '"
    )
    while time.time() < deadline:
        result = host.run(f"sudo -n /bin/sh -c {shlex.quote(probe)}")
        if result.rc == 0:
            return
        time.sleep(0.5)
    raise AssertionError("bare xray-core fixture did not become ready")
def stop(host) -> None:
    script = (
        f"for f in {ROOT}/xray.pid {ROOT}/http.pid; do "
        "[ -f \"$f\" ] || continue; pid=$(cat \"$f\"); "
        "kill \"$pid\" 2>/dev/null || true; "
        "for i in 1 2 3 4 5; do kill -0 \"$pid\" 2>/dev/null || break; sleep 1; done; "
        "kill -9 \"$pid\" 2>/dev/null || true; done; "
        f"rm -rf {ROOT}"
    )
    host.run(f"sudo -n /bin/sh -c {shlex.quote(script)}")
@contextmanager
def running(server_host):
    stop(server_host)
    config = _encoded(_server_config())
    marker = _encoded({"fixture": "bare-xray-core", "route": "local"})
    script = (
        f"install -d -m 0700 {ROOT}; "
        f"echo {shlex.quote(config)} | base64 -d > {ROOT}/config.json; "
        f"install -d -m 0755 {ROOT}/www; "
        f"echo {shlex.quote(marker)} | base64 -d > {ROOT}/www/route.json; "
        f"nohup python3 -m http.server {LOCAL_HTTP_PORT} --bind 127.0.0.1 --directory {ROOT}/www "
        f">{ROOT}/http.log 2>&1 & echo $! > {ROOT}/http.pid; "
        f"nohup {XRAY} run -config {ROOT}/config.json >{ROOT}/xray.log 2>&1 & echo $! > {ROOT}/xray.pid"
    )
    result = server_host.run(f"sudo -n /bin/sh -c {shlex.quote(script)}")
    assert result.rc == 0, result.stderr
    failed = False
    try:
        _wait_ready(server_host)
        yield
    except Exception:
        failed = True
        script = (
            f"for f in {ROOT}/xray.pid {ROOT}/http.pid; do "
            "[ -f \"$f\" ] && kill $(cat \"$f\") 2>/dev/null || true; done"
        )
        server_host.run(f"sudo -n /bin/sh -c {shlex.quote(script)}")
        raise
    finally:
        if not failed:
            stop(server_host)
def wait_for_socks(client_host, *, should_succeed: bool = True) -> None:
    command = (
        f"curl --fail --silent --show-error --max-time 8 --socks5-hostname {SOCKS_ADDRESS} "
        f"http://127.0.0.1:{LOCAL_HTTP_PORT}/route.json"
    )
    deadline = time.time() + 30
    last = None
    while time.time() < deadline:
        listener = client_host.run("ss -ltn | grep -q ':51180 '")
        if listener.rc != 0:
            time.sleep(1)
            continue
        last = client_host.run(command)
        if (last.rc == 0) == should_succeed:
            return
        time.sleep(1)
    expectation = "succeed" if should_succeed else "fail"
    raise AssertionError(f"expected SOCKS request to {expectation}; last exit={last.rc if last else 'none'}")


def assert_two_traffic_paths(client_host) -> None:
    local = client_host.run(
        f"curl --fail --silent --show-error --max-time 10 --socks5-hostname {SOCKS_ADDRESS} "
        f"http://127.0.0.1:{LOCAL_HTTP_PORT}/route.json"
    )
    assert local.rc == 0, local.stderr
    assert json.loads(local.stdout)["route"] == "local"

    direct = client_host.run("curl --fail --silent --show-error --max-time 10 https://api.ipify.org")
    tunneled = client_host.run(
        f"curl --fail --silent --show-error --max-time 15 --socks5-hostname {SOCKS_ADDRESS} {EXTERNAL_IP_URL}"
    )
    assert direct.rc == 0, direct.stderr
    assert tunneled.rc == 0, tunneled.stderr
    direct_ip = direct.stdout.strip()
    tunneled_ip = tunneled.stdout.strip()
    assert tunneled_ip, "external IP service returned an empty response"
    assert tunneled_ip == direct_ip, f"unexpected bare-server egress IP: {tunneled_ip} != {direct_ip}"


def state_digest(host, paths: list[PurePosixPath]) -> str:
    digest = hashlib.sha256()
    for path in paths:
        result = host.run(f"sudo -n sha256sum {path}")
        assert result.rc == 0, result.stderr
        digest.update(result.stdout.encode())
    return digest.hexdigest()


def failure_dump(client_host, server_host) -> None:
    for host, label in ((client_host, "client"), (server_host, "bare-server")):
        print(f"==== BARE XRAY FAILURE DUMP ({label}) ====")
        result = host.run(
            f"sudo -n /bin/sh -c \"find /etc/xp2p/.state -maxdepth 5 -print 2>/dev/null; "
            f"find {ROOT} -maxdepth 2 -type f -print -exec tail -n 100 {{}} \\; 2>/dev/null; "
            "systemctl --no-pager --full status xp2p-client.service 2>/dev/null || true; "
            "journalctl --no-pager -u xp2p-client.service -n 150 2>/dev/null || true\""
        )
        output = (result.stdout or "")[-20000:]
        print(output.encode("ascii", errors="backslashreplace").decode("ascii"))
