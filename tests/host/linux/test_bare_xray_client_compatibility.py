from __future__ import annotations

import json

import pytest

from . import _bare_xray as bare
from . import _helpers as helpers
from . import _heartbeat_sidecar as heartbeat_sidecar
from . import env as linux_env
from tests.host.host_common.polling import wait_until


STATE_PATHS = [
    helpers.CLIENT_CONFIG_FILE,
    helpers.CLIENT_LIVE_DIR / "xray.json",
]


def _install(client_host, runner, server_host, protocol: str, *, pin: str | None = None, verify_name: str = bare.TLS_NAME):
    linux_env.run_guest_script(client_host, "scripts/linux/update_hosts_entry.sh", "add", bare.SERVER_IP, bare.TLS_NAME)
    actual_pin = pin if pin is not None else bare.certificate_pin(server_host)
    runner(
        "client", "install",
        "--path", helpers.INSTALL_ROOT.as_posix(),
        "--config-dir", helpers.CLIENT_CONFIG_DIR_NAME,
        "--link", bare.connection_link(protocol, actual_pin, verify_name=verify_name),
        "--force",
        check=True,
    )
    tag = helpers.expected_proxy_tag(bare.TLS_NAME)
    runner("client", "redirect", "add", "--cidr", "127.0.0.1/32", "--tag", tag, "--no-routes", check=True)
    runner("client", "redirect", "add", "--domain", "api.ipify.org", "--tag", tag, check=True)


def _assert_profile(client_host, runner, protocol: str) -> None:
    desired = helpers.render_xray(client_host, runner, "client", desired=True)
    live = helpers.render_xray(client_host, runner, "client", desired=False)
    pin = bare.certificate_pin(client_host)
    credential = bare.TROJAN_PASSWORD if protocol == "trojan" else bare.VLESS_UUID
    for document in (desired, live):
        outbound = next(item for item in document["outbounds"] if item.get("protocol") == protocol)
        assert outbound["streamSettings"]["tlsSettings"]["serverName"] == bare.TLS_NAME
        assert outbound["streamSettings"]["tlsSettings"]["verifyPeerCertByName"] == bare.TLS_NAME
        assert outbound["streamSettings"]["tlsSettings"]["pinnedPeerCertSha256"] == pin
        server = (outbound["settings"].get("servers") or outbound["settings"].get("vnext"))[0]
        assert server["address"] == bare.SERVER_IP
        user = (server.get("users") or [server])[0]
        assert user.get("password") == credential or user.get("id") == credential
        if protocol == "vless":
            assert user["flow"] == "xtls-rprx-vision"
        rules = document["routing"]["rules"]
        assert any(rule.get("ip") == ["127.0.0.1/32"] and rule.get("outboundTag") == outbound["tag"] for rule in rules)
        assert any("domain:api.ipify.org" in (rule.get("domain") or rule.get("domains") or []) and rule.get("outboundTag") == outbound["tag"] for rule in rules)


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize("protocol", ["trojan", "vless"])
def test_direct_link_with_bare_xray_routes_local_and_internet(client_host, server_host, xp2p_client_runner, protocol):
    try:
        with bare.running(server_host):
            _install(client_host, xp2p_client_runner, server_host, protocol)
            xp2p_client_runner("client", "service", "start", check=True)
            bare.wait_for_socks(client_host)
            _assert_profile(client_host, xp2p_client_runner, protocol)
            bare.assert_two_traffic_paths(client_host)
            _assert_heartbeat_status(client_host, "not-detected", "auto")
            with heartbeat_sidecar.late_sidecar(server_host, protocol):
                _assert_heartbeat_status(client_host, "healthy", "auto")
            _assert_heartbeat_status(client_host, "unhealthy", "auto")

            xp2p_client_runner("client", "service", "restart", check=True)
            bare.wait_for_socks(client_host)
            _assert_profile(client_host, xp2p_client_runner, protocol)
            bare.assert_two_traffic_paths(client_host)
            _assert_heartbeat_status(client_host, "unhealthy", "auto")
    except Exception:
        bare.failure_dump(client_host, server_host)
        raise
    finally:
        bare.stop(server_host)
        linux_env.run_guest_script(client_host, "scripts/linux/update_hosts_entry.sh", "remove", bare.TLS_NAME)


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize(
    ("pin", "verify_name"),
    [("00" * 32, bare.TLS_NAME), ("", "wrong-name.example.test")],
)
def test_bare_xray_rejects_invalid_tls_identity(client_host, server_host, xp2p_client_runner, pin, verify_name):
    try:
        with bare.running(server_host):
            _install(client_host, xp2p_client_runner, server_host, "trojan", pin=pin, verify_name=verify_name)
            xp2p_client_runner("client", "service", "start", check=True)
            bare.wait_for_socks(client_host, should_succeed=False)
    except Exception:
        bare.failure_dump(client_host, server_host)
        raise
    finally:
        bare.stop(server_host)
        linux_env.run_guest_script(client_host, "scripts/linux/update_hosts_entry.sh", "remove", bare.TLS_NAME)


@pytest.mark.host
@pytest.mark.linux
def test_unsupported_direct_link_preserves_desired_and_live(client_host, server_host, xp2p_client_runner):
    secret = "credential-that-must-not-leak"
    try:
        with bare.running(server_host):
            _install(client_host, xp2p_client_runner, server_host, "trojan")
            xp2p_client_runner("client", "service", "start", check=True)
            bare.wait_for_socks(client_host)
            before = bare.state_digest(client_host, STATE_PATHS)
            invalid = (
                f"trojan://{secret}@{bare.TLS_NAME}:{bare.TROJAN_PORT}?"
                f"security=tls&type=tcp&sni={bare.TLS_NAME}&requiredFeature=unsupported"
            )
            result = xp2p_client_runner(
                "client", "install",
                "--path", helpers.INSTALL_ROOT.as_posix(),
                "--config-dir", helpers.CLIENT_CONFIG_DIR_NAME,
                "--link", invalid,
                "--force",
            )
            output = (result.stdout or "") + (result.stderr or "")
            assert result.rc != 0
            assert "unsupported" in output.lower()
            assert secret not in output
            assert bare.state_digest(client_host, STATE_PATHS) == before
            bare.assert_two_traffic_paths(client_host)
    except Exception:
        bare.failure_dump(client_host, server_host)
        raise
    finally:
        bare.stop(server_host)
        linux_env.run_guest_script(client_host, "scripts/linux/update_hosts_entry.sh", "remove", bare.TLS_NAME)


def _assert_heartbeat_status(client_host, status: str, mode: str) -> None:
    def observed():
        result = client_host.run(f"cat {helpers.CLIENT_HEARTBEAT_STATE_FILE}")
        if result.rc != 0:
            return None
        entries = (json.loads(result.stdout or "{}").get("entries") or {}).values()
        return next(
            (
                entry
                for entry in entries
                if entry.get("host") == bare.TLS_NAME
                and entry.get("status") == status
                and entry.get("mode") == mode
            ),
            None,
        )

    try:
        wait_until(
            f"bare Xray heartbeat status {status}",
            observed,
            timeout_seconds=30.0,
            poll_interval=1.0,
        )
    except TimeoutError:
        bare.failure_dump(client_host, client_host)
        raise
