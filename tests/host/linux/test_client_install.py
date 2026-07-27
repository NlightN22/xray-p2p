from __future__ import annotations

from pathlib import PurePosixPath
import time

import pytest

from tests.host import cli_json
from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env
from tests.host.tunnel import common as tunnel_common

CLIENT_LOG_PATH = helpers.CLIENT_LOG_FILE
CLIENT_STATE_FILE = helpers.CLIENT_CONFIG_FILE


@pytest.mark.host
@pytest.mark.linux
def test_client_install_and_force_overwrites(client_host, xp2p_client_runner):
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.10",
            "--user",
            "alpha@example.com",
            "--password",
            "test_password123",
            check=True,
        )

        data = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_outbound(
            data,
            "10.55.0.10",
            "test_password123",
            "alpha@example.com",
            "10.55.0.10",
        )

        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.11",
            "--user",
            "beta@example.com",
            "--password",
            "override_password456",
            "--sni",
            "vpn.example.local",
            check=True,
        )

        updated = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_outbound(
            updated,
            "10.55.0.10",
            "test_password123",
            "alpha@example.com",
            "10.55.0.10",
            allow_insecure=False,
        )
        helpers.assert_outbound(
            updated,
            "10.55.0.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=False,
        )

        routing = updated
        helpers.assert_routing_rule(routing, "10.55.0.10")
        helpers.assert_routing_rule(routing, "10.55.0.11")

        state = helpers.read_pending_client_config(client_host)
        recorded_hosts = {entry["hostname"] for entry in state.get("endpoints", [])}
        assert recorded_hosts == {"10.55.0.10", "10.55.0.11"}

        before_duplicate_sha = helpers.file_sha256(client_host, helpers.CLIENT_CONFIG_FILE)
        duplicate = xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.10",
            "--user",
            "gamma@example.com",
            "--password",
            "newpass",
            check=False,
        )
        assert duplicate.rc != 0, "Expected duplicate endpoint install to fail without --force"
        combined = (duplicate.stderr or "") + (duplicate.stdout or "")
        combined = combined.lower()
        assert "endpoint 10.55.0.10" in combined and "already exists" in combined
        assert helpers.file_sha256(client_host, helpers.CLIENT_CONFIG_FILE) == before_duplicate_sha

        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.10",
            "--user",
            "gamma@example.com",
            "--password",
            "forcepass",
            "--sni",
            "override.linux",
            "--force",
            check=True,
        )

        refreshed = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_outbound(
            refreshed,
            "10.55.0.10",
            "forcepass",
            "gamma@example.com",
            "override.linux",
            allow_insecure=False,
        )
        helpers.assert_outbound(
            refreshed,
            "10.55.0.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=False,
        )
    finally:
        if helpers.path_exists(client_host, helpers.HEARTBEAT_STATE_FILE):
            helpers.remove_path(client_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_from_link(client_host, xp2p_client_runner):
    try:
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "add",
            "198.51.100.44",
            "link.example.test",
        )
        link = (
            "trojan://linkpass@link.example.test:58443?"
            "pinnedPeerCertSha256=deadbeef&security=tls&sni=link.example.test&"
            "verifyPeerCertByName=link.example.test#link@example.com"
        )
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link,
            "--force",
            check=True,
        )
        data = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_outbound(
            data,
            "link.example.test",
            "linkpass",
            "link@example.com",
            "link.example.test",
            address="198.51.100.44",
            pinned_peer_sha256="",
            verify_peer_name="link.example.test",
        )
    finally:
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "remove",
            "link.example.test",
        )


@pytest.mark.host
@pytest.mark.linux
def test_client_install_from_link_without_allow_insecure(client_host, xp2p_client_runner):
    try:
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "add",
            "198.51.100.44",
            "link.example.test",
        )
        link = (
            "trojan://linkpass@link.example.test:58443?"
            "security=tls&sni=link.example.test#link@example.com"
        )
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link,
            "--force",
            check=True,
        )
        data = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_outbound(
            data,
            "link.example.test",
            "linkpass",
            "link@example.com",
            "link.example.test",
            address="198.51.100.44",
            allow_insecure=False,
        )
    finally:
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "remove",
            "link.example.test",
        )


@pytest.mark.host
@pytest.mark.linux
def test_client_state_reports_multiple_endpoints(client_host, xp2p_client_runner):
    try:
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "add",
            "198.51.100.44",
            "link.example.test",
        )
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.40",
            "--user",
            "state-one@example.com",
            "--password",
            "state-pass-one",
            check=True,
        )
        link = (
            "trojan://statepass@link.example.test:58443?"
            "pinnedPeerCertSha256=deadbeef&security=tls&sni=link.example.test&"
            "verifyPeerCertByName=link.example.test#state-two@example.com"
        )
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link,
            check=True,
        )

        result = xp2p_client_runner(
            "client",
            "state",
            "--json",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--pending",
            check=True,
        )
        rows = tunnel_common.parse_state_result(result.stdout or "")
        expected_tags = {
            helpers.expected_proxy_tag("10.55.0.40"),
            helpers.expected_proxy_tag("link.example.test"),
        }
        expected_hosts = {"10.55.0.40", "link.example.test"}
        assert len(rows) == 2
        assert {row["TAG"] for row in rows} == expected_tags
        assert {row["HOST"] for row in rows} == expected_hosts
    except Exception:
        helpers.dump_failure_state(client_host, "client-state-multiple-endpoints")
        raise
    finally:
        xp2p_client_runner("client", "service", "stop")
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "remove",
            "link.example.test",
        )


def _assert_no_endpoint(host: str, data: dict):
    tag = helpers.expected_proxy_tag(host)
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            pytest.fail(f"Unexpected outbound {tag} still present")


@pytest.mark.host
@pytest.mark.linux
def test_client_remove_endpoint_and_list(client_host, xp2p_client_runner):
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.66.0.10",
            "--user",
            "delta@example.com",
            "--password",
            "delta-pass",
            check=True,
        )
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.66.0.11",
            "--user",
            "echo@example.com",
            "--password",
            "echo-pass",
            check=True,
        )

        list_result = xp2p_client_runner(
            "client",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--pending",
            check=True,
        ).stdout or ""
        assert "HOSTNAME" in list_result
        assert "10.66.0.10" in list_result
        assert "10.66.0.11" in list_result

        redirect_cidr = "10.200.0.0/16"
        host_tag = helpers.expected_proxy_tag("10.66.0.10")
        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            redirect_cidr,
            "--tag",
            host_tag,
            check=True,
        )
        redirect_list = xp2p_client_runner(
            "client",
            "redirect",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--pending",
            "--json",
            check=True,
        ).stdout or ""
        assert any(item.get("value") == redirect_cidr for item in cli_json.result(redirect_list).get("redirects", []))

        xp2p_client_runner(
            "client",
            "redirect",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            redirect_cidr,
            check=True,
        )
        redirect_list_after_remove = xp2p_client_runner(
            "client",
            "redirect",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--pending",
            "--json",
            check=True,
        ).stdout or ""
        assert all(
            item.get("value") != redirect_cidr
            for item in cli_json.result(redirect_list_after_remove).get("redirects", [])
        )

        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            redirect_cidr,
            "--tag",
            host_tag,
            check=True,
        )
        xp2p_client_runner(
            "client",
            "update",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--user",
            "delta-updated@example.com",
            "--password",
            "delta-updated-pass",
            "10.66.0.10",
            check=True,
        )
        updated = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_outbound(
            updated,
            "10.66.0.10",
            "delta-updated-pass",
            "delta-updated@example.com",
            "10.66.0.10",
        )
        helpers.assert_redirect_rule(updated, redirect_cidr, host_tag)

        xp2p_client_runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "10.66.0.10",
            check=True,
        )

        try:
            outbounds = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
            helpers.assert_outbound(
                outbounds,
                "10.66.0.11",
                "echo-pass",
                "echo@example.com",
                "10.66.0.11",
            )
            _assert_no_endpoint("10.66.0.10", outbounds)

            routing = outbounds
            helpers.assert_routing_rule(routing, "10.66.0.11")

            state = helpers.read_pending_client_config(client_host)
            hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
            assert hosts == {"10.66.0.11"}

            redirect_list_after = xp2p_client_runner(
                "client",
                "redirect",
                "list",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--pending",
                "--json",
                check=True,
            ).stdout or ""
            assert all(
                item.get("value") != redirect_cidr
                for item in cli_json.result(redirect_list_after).get("redirects", [])
            )

            list_after = xp2p_client_runner(
                "client",
                "list",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--pending",
                check=True,
            ).stdout or ""
            assert "10.66.0.11" in list_after
            assert "10.66.0.10" not in list_after
        except BaseException as exc:
            if isinstance(exc, KeyboardInterrupt):
                raise
            debug = []
            for path in (CLIENT_STATE_FILE,):
                if helpers.path_exists(client_host, path):
                    tail = "\n".join((helpers.read_text(client_host, path) or "").splitlines()[-200:])
                    debug.append(f"{path}:\n{tail}")
            list_debug = xp2p_client_runner(
                "client",
                "list",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--pending",
                check=False,
            )
            redirect_debug = xp2p_client_runner(
                "client",
                "redirect",
                "list",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--pending",
                check=False,
            )
            debug.append(f"xp2p client list rc={list_debug.rc}\n{list_debug.stdout or ''}\n{list_debug.stderr or ''}")
            debug.append(
                f"xp2p client redirect rc={redirect_debug.rc}\n{redirect_debug.stdout or ''}\n{redirect_debug.stderr or ''}"
            )
            raise AssertionError(
                f"{exc}\n\nDebug details:\n" + "\n\n".join(debug)
            ) from exc

        xp2p_client_runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--all",
            check=True,
        )

        final_list = xp2p_client_runner(
            "client",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--pending",
            check=True,
        ).stdout or ""
        assert "No client endpoints configured." in final_list
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_client_install_rejects_same_endpoint_without_desired_update(client_host, xp2p_client_runner):
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.20",
            "--user",
            "state@example.com",
            "--password",
            "state-pass",
            check=True,
        )

        apply_request = helpers.CONFIG_ROOT / helpers.APPLY_DIR_NAME / "apply.request"
        helpers.remove_path(client_host, apply_request)
        assert not helpers.path_exists(client_host, apply_request)
        before_duplicate_sha = helpers.file_sha256(client_host, helpers.CLIENT_CONFIG_FILE)
        result = xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.20",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            check=False,
        )
        assert result.rc != 0, "Expected duplicate endpoint install to fail without --force"
        combined = f"{result.stdout}\n{result.stderr}".lower()
        assert "endpoint 10.55.0.20" in combined and "already exists" in combined
        assert helpers.file_sha256(client_host, helpers.CLIENT_CONFIG_FILE) == before_duplicate_sha
        assert not helpers.path_exists(client_host, apply_request)

        state = helpers.read_pending_client_config(client_host)
        endpoints = [entry for entry in state.get("endpoints", []) if entry.get("hostname") == "10.55.0.20"]
        assert len(endpoints) == 1
        assert endpoints[0]["user"] == "state@example.com"
        assert endpoints[0]["password"] == "state-pass"
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_client_install_recovers_without_state_marker(client_host, xp2p_client_runner):
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.30",
            "--user",
            "nostate@example.com",
            "--password",
            "nostate-pass",
            "--force",
            check=True,
        )

        for state_file in helpers.CLIENT_STATE_FILES:
            helpers.remove_path(client_host, state_file)
            assert not helpers.path_exists(client_host, state_file)

        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.31",
            "--user",
            "nostate2@example.com",
            "--password",
            "nostate-pass-2",
            check=True,
        )

        desired_config = helpers.CLIENT_CONFIG_FILE
        assert helpers.path_exists(client_host, desired_config), "Expected desired client config to be recreated"
    finally:
        pass


def _wait_for_apply_request_clear(host, *, timeout_seconds: float) -> None:
    deadline = time.time() + timeout_seconds
    apply_path = helpers.CONFIG_ROOT / helpers.APPLY_DIR_NAME / "apply.request"
    while time.time() < deadline:
        if not linux_env.path_exists(host, apply_path):
            return
        time.sleep(1.0)
    raise AssertionError(f"apply.request not cleared within {timeout_seconds:.0f}s at {apply_path}")


def _wait_for_path_present(host, path: PurePosixPath, *, timeout_seconds: float) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if linux_env.path_exists(host, path):
            return
        time.sleep(1.0)
    raise AssertionError(f"Expected path {path} to exist within {timeout_seconds:.0f}s")
