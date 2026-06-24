from __future__ import annotations

import json
import time
from urllib.parse import parse_qs, urlparse

import pytest
from testinfra.host import Host

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env


GROUP_ID = "ha-linux"
GROUP_TAG = "ha-linux-group"
CLIENT_GROUP_ID = "ha-client-linux"
CLIENT_GROUP_TAG = "ha-client-group"
SECRET = "ha-shared-secret"
HOST_ONLY_IPS = {
    linux_env.DEFAULT_CLIENT: "10.62.10.11",
    linux_env.DEFAULT_SERVER: "10.62.10.12",
    linux_env.DEFAULT_AUX: "10.62.10.13",
}


def runner(host: Host):
    def run(*args: str, check: bool = False):
        result = linux_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return run


def server_install(run, host: str, port: str) -> None:
    run(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--port",
        port,
        "--host",
        host,
        "--force",
        check=True,
    )


def client_install(run, host: str, port: str, user: str, password: str) -> None:
    run(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--host",
        host,
        "--port",
        port,
        "--user",
        user,
        "--password",
        password,
        "--allow-insecure",
        "--mode",
        "proxy",
        check=True,
    )


def client_install_link(run, link: str) -> None:
    run(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--link",
        link,
        "--mode",
        "proxy",
        check=True,
    )


def link_tls_metadata(link: str) -> tuple[str, str]:
    parsed = urlparse(link)
    params = parse_qs(parsed.query)
    server_name = (params.get("sni") or [""])[0]
    pin = (params.get("xp2p_pin_sha256") or [""])[0]
    return server_name, pin


def ha(run, *args: str, check: bool = True):
    return run("server", "ha", *args, check=check)


def control_endpoint(ip: str) -> str:
    return f"https://{ip}:{helpers.SERVER_DIAG_PORT}"


def generation(host: Host) -> dict:
    server = helpers.read_toml(host, helpers.SERVER_CONFIG_FILE).get("server") or {}
    return server.get("ha_generation") or {}


def generation_number(host: Host) -> int:
    return int(generation(host).get("number") or 0)


def assert_generation_member(host: Host, member_id: str) -> None:
    members = ((generation(host).get("group") or {}).get("members") or [])
    assert any(item.get("id") == member_id and item.get("confirmed") is True for item in members), members


def assert_generation_tombstone(host: Host, member_id: str) -> None:
    members = ((generation(host).get("group") or {}).get("members") or [])
    assert any(item.get("id") == member_id and item.get("tombstone") is True for item in members), members


def assert_client_has_no_endpoint_groups(host: Host) -> None:
    client = helpers.read_toml(host, helpers.CLIENT_CONFIG_FILE).get("client") or {}
    assert not client.get("endpoint_groups"), client.get("endpoint_groups")


def assert_server_live_subscription_topology(host: Host, group_tag: str) -> None:
    runtime_doc = helpers.read_json(host, helpers.SERVER_LIVE_DIR / "runtime.json")
    subscription = ((runtime_doc.get("control") or {}).get("subscription") or {})
    topology = subscription.get("topology") or {}
    group = topology.get("group") or {}
    assert group.get("tag") == group_tag, subscription


def wait_for_group_active(run, expected_tag: str, timeout_seconds: float = 35.0) -> str:
    deadline = time.time() + timeout_seconds
    last_output = ""
    while time.time() < deadline:
        result = run("client", "group", "list", check=False)
        last_output = f"{result.stdout}\n{result.stderr}"
        if result.rc == 0 and _group_active_tag(last_output) == expected_tag:
            return last_output
        time.sleep(1.5)
    raise AssertionError(f"Endpoint group did not become active on {expected_tag}.\n{last_output}")


def client_endpoint_group_debug(host: Host) -> str:
    parts: list[str] = []
    for label, path in (
        ("selector", helpers.CLIENT_LIVE_DIR / "endpoint-selector.json"),
        ("selector journal", helpers.CLIENT_LIVE_DIR / "endpoint-selector.journal.json"),
        ("heartbeat", helpers.CLIENT_HEARTBEAT_STATE_FILE),
        ("runtime", helpers.CLIENT_LIVE_DIR / "runtime.json"),
        ("xray", helpers.CLIENT_LIVE_DIR / "xray.json"),
        ("client service log", helpers.LOG_ROOT / "client" / "service.log"),
        ("apply request", helpers.STATE_ROOT / "apply.request"),
        ("apply error", helpers.STATE_ROOT / "apply.error"),
    ):
        result = host.run(f"sudo -n cat {path.as_posix()} 2>/dev/null || true")
        content = (result.stdout or "").strip()
        if not content:
            parts.append(f"== {label} ==\n<missing>")
            continue
        if label == "runtime":
            content = _compact_runtime(content)
        elif label == "xray":
            content = _compact_xray(content)
        elif label == "client service log":
            content = _tail_lines(content, 160)
        parts.append(f"== {label} ==\n{content}")
    parts.append(f"== subscription probe ==\n{client_subscription_probe(host)}")
    return "\n\n".join(parts)


def client_subscription_probe(host: Host) -> str:
    script = r'''
import hashlib
import hmac
import json
import ssl
import time
import urllib.request

runtime_path = "/etc/xp2p/.state/live/config-client/runtime.json"
with open(runtime_path, "r", encoding="utf-8") as fh:
    meta = json.load(fh)
endpoint = (meta.get("desired", {}).get("endpoints") or [{}])[0]
auth = (meta.get("control", {}).get("auth_users") or [{}])[0]
host = endpoint.get("hostname") or endpoint.get("address")
url = "https://%s:62022/control/v1/subscription" % host
path = "/control/v1/subscription"
nonce = str(time.time_ns())
ts = str(int(time.time()))
body_hash = hashlib.sha256(b"").hexdigest()
canonical = "GET\n%s\n\n%s\n%s\n%s" % (path, body_hash, ts, nonce)
secret = auth.get("credential", "")
sig = hmac.new(secret.encode(), canonical.encode(), hashlib.sha256).hexdigest()
req = urllib.request.Request(url, method="GET", headers={
    "X-XP2P-User": auth.get("label", ""),
    "X-XP2P-Timestamp": ts,
    "X-XP2P-Nonce": nonce,
    "X-XP2P-Signature": sig,
})
try:
    with urllib.request.urlopen(req, timeout=5, context=ssl._create_unverified_context()) as resp:
        print("status=%s" % resp.status)
        print(resp.read().decode("utf-8", "replace"))
except Exception as exc:
    print("error=%s: %s" % (type(exc).__name__, exc))
'''
    result = host.run("sudo -n python3 - <<'PY'\n" + script + "\nPY")
    return f"rc={result.rc}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"


def server_subscription_debug(host: Host) -> str:
    parts: list[str] = []
    for label, path in (
        ("server desired", helpers.SERVER_CONFIG_FILE),
        ("server runtime", helpers.SERVER_LIVE_DIR / "runtime.json"),
        ("server service log", helpers.LOG_ROOT / "server" / "service.log"),
    ):
        result = host.run(f"sudo -n cat {path.as_posix()} 2>/dev/null || true")
        content = (result.stdout or "").strip()
        if not content:
            parts.append(f"== {label} ==\n<missing>")
            continue
        if label == "server runtime":
            content = _compact_server_runtime(content)
        elif label == "server service log":
            content = _tail_lines(content, 160)
        parts.append(f"== {label} ==\n{content}")
    listeners = host.run("sudo -n ss -lntp 2>/dev/null | grep -E ':(62022|62191|62192) ' || true")
    parts.append(f"== listeners ==\n{(listeners.stdout or '').strip() or '<none>'}")
    return "\n\n".join(parts)


def server_control_probe(host: Host, server_ip: str) -> str:
    script = r'''
import socket
import ssl
import urllib.request

server_ip = "__SERVER_IP__"
try:
    with socket.create_connection((server_ip, 62022), timeout=5):
        print("tcp=connected")
except Exception as exc:
    print("tcp_error=%s: %s" % (type(exc).__name__, exc))

try:
    url = "https://%s:62022/control/v1/subscription" % server_ip
    with urllib.request.urlopen(url, timeout=5, context=ssl._create_unverified_context()) as resp:
        print("https_status=%s" % resp.status)
        print(resp.read(256).decode("utf-8", "replace"))
except Exception as exc:
    print("https_error=%s: %s" % (type(exc).__name__, exc))
'''
    script = script.replace("__SERVER_IP__", server_ip)
    result = host.run("sudo -n python3 - <<'PY'\n" + script + "\nPY")
    return f"rc={result.rc}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"


def _compact_runtime(content: str) -> str:
    try:
        doc = json.loads(content)
    except json.JSONDecodeError:
        return content
    desired = doc.get("desired") or {}
    compact = {
        "control": doc.get("control"),
        "endpoints": desired.get("endpoints"),
        "endpoint_groups": desired.get("endpoint_groups"),
    }
    return json.dumps(compact, indent=2, sort_keys=True)


def _compact_xray(content: str) -> str:
    try:
        doc = json.loads(content)
    except json.JSONDecodeError:
        return content
    compact = {
        "outbounds": [
            {
                "tag": item.get("tag"),
                "protocol": item.get("protocol"),
                "address": ((item.get("settings") or {}).get("servers") or [{}])[0].get("address"),
                "port": ((item.get("settings") or {}).get("servers") or [{}])[0].get("port"),
                "serverName": ((item.get("streamSettings") or {}).get("tlsSettings") or {}).get("serverName"),
                "pinnedPeerCertificateChainSha256": ((item.get("streamSettings") or {}).get("tlsSettings") or {}).get("pinnedPeerCertificateChainSha256"),
            }
            for item in doc.get("outbounds") or []
        ],
        "diagnostics_rules": [
            item
            for item in ((doc.get("routing") or {}).get("rules") or [])
            if "diagnostics" in str(item.get("ruleTag") or "")
        ],
    }
    return json.dumps(compact, indent=2, sort_keys=True)


def _compact_server_runtime(content: str) -> str:
    try:
        doc = json.loads(content)
    except json.JSONDecodeError:
        return content
    return json.dumps({"control": doc.get("control")}, indent=2, sort_keys=True)


def _tail_lines(content: str, limit: int) -> str:
    lines = content.splitlines()
    if len(lines) <= limit:
        return content
    return "\n".join(lines[-limit:])


def _group_active_tag(output: str) -> str:
    for raw in (output or "").splitlines():
        columns = raw.split()
        if len(columns) >= 4 and columns[1] == CLIENT_GROUP_TAG:
            return columns[3]
    return ""


def assert_tunnel_ping(run, host: str, endpoint_tag: str) -> None:
    result = run("ping", host, "--tunnel", "--endpoint", endpoint_tag, "--count", "3", check=False)
    output = f"{result.stdout}\n{result.stderr}".lower()
    assert result.rc == 0 and "0% loss" in output, output
