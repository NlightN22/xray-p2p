from __future__ import annotations

import json
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as runtime
from tests.host.linux import _sessions
from tests.host.linux import env as linux_env
from tests.host.linux import _ha_control_plane_helpers as ha_helpers

HA_REDIRECT_IP = "10.77.0.10"
HA_REDIRECT_CIDR = f"{HA_REDIRECT_IP}/32"
HA_SERVER_REDIRECT_CIDR = "10.77.20.0/24"
HA_REVERSE_TAG = "ha-client-portal.rev"

@pytest.mark.host
@pytest.mark.linux
def test_ha_control_plane_commits_replacement_tombstone_and_force_paths(linux_host_factory):
    active_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    lost_host = linux_host_factory(linux_env.DEFAULT_CLIENT)
    witness_host = linux_host_factory(linux_env.DEFAULT_AUX)

    active = ha_helpers.runner(active_host)
    lost = ha_helpers.runner(lost_host)
    witness = ha_helpers.runner(witness_host)

    active_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_SERVER]
    lost_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_CLIENT]
    witness_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_AUX]

    ha_helpers.server_install(active, active_ip, "62171")
    ha_helpers.server_install(lost, lost_ip, "62172")
    ha_helpers.server_install(witness, witness_ip, "62173")

    ha_helpers.ha(active, "group", "create", ha_helpers.GROUP_ID, ha_helpers.GROUP_TAG)
    ha_helpers.ha(active, "member", "add", "active", "active-endpoint", active_ip, "62171", "trojan-tls")

    ha_helpers.ha(witness, "peer", "self", "witness")
    ha_helpers.ha(witness, "peer", "add", "active", ha_helpers.control_endpoint(active_ip), ha_helpers.SECRET, "--allow-insecure")

    with _sessions.xp2p_run_session(witness_host, "server", helpers.INSTALL_ROOT, helpers.SERVER_CONFIG_DIR_NAME):
        ha_helpers.ha(active, "peer", "self", "active")
        ha_helpers.ha(active, "peer", "add", "lost", ha_helpers.control_endpoint(lost_ip), ha_helpers.SECRET, "--allow-insecure")
        ha_helpers.ha(
            active,
            "peer",
            "add",
            "witness",
            ha_helpers.control_endpoint(witness_ip),
            ha_helpers.SECRET,
            "--allow-insecure",
            "--witness",
        )

        ha_helpers.ha(active, "channel", "create", "ha-portal", "ha-portal", "portal.ha.internal")
        ha_helpers.ha(active, "redirect", "add", "ha-portal", "--cidr", "10.77.10.0/24")
        assert "10.77.10.0/24" in (ha_helpers.ha(active, "redirect", "list").stdout or "")
        assert "10.77.10.0/24" in (ha_helpers.ha(witness, "redirect", "list").stdout or "")
        ha_helpers.ha(active, "redirect", "remove", "ha-portal", "--cidr", "10.77.10.0/24")
        assert "10.77.10.0/24" not in (ha_helpers.ha(active, "redirect", "list").stdout or "")
        assert "10.77.10.0/24" not in (ha_helpers.ha(witness, "redirect", "list").stdout or "")

        ha_helpers.ha(active, "member", "add", "replacement", "replacement-endpoint", lost_ip, "62172", "trojan-tls")
        ha_helpers.ha(active, "member", "remove", "active")

        ha_helpers.assert_generation_member(active_host, "replacement")
        ha_helpers.assert_generation_member(witness_host, "replacement")
        ha_helpers.assert_generation_tombstone(active_host, "active")
        ha_helpers.assert_generation_tombstone(witness_host, "active")
        assert ha_helpers.generation_number(active_host) == ha_helpers.generation_number(witness_host)

        status = ha_helpers.ha(active, "status").stdout or ""
        assert "Coordinator:" in status
        assert "Voting membership:" in status
        assert "Quorum: 2" in status

    before = ha_helpers.generation_number(active_host)
    blocked = ha_helpers.ha(
        active,
        "member",
        "add",
        "blocked",
        "blocked-endpoint",
        lost_ip,
        "62174",
        "trojan-tls",
        check=False,
    )
    assert blocked.rc != 0
    assert "quorum is unavailable" in f"{blocked.stdout}\n{blocked.stderr}".lower()
    assert ha_helpers.generation_number(active_host) == before

    missing_reason = ha_helpers.ha(
        active,
        "member",
        "add",
        "forced-missing-reason",
        "forced-missing-reason",
        lost_ip,
        "62175",
        "trojan-tls",
        "--force",
        check=False,
    )
    assert missing_reason.rc != 0
    assert "force-reconfiguration is not authorized" in f"{missing_reason.stdout}\n{missing_reason.stderr}".lower()


@pytest.mark.host
@pytest.mark.linux
def test_ha_control_plane_rejects_non_voting_peer_as_quorum(linux_host_factory):
    active_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    observer_host = linux_host_factory(linux_env.DEFAULT_AUX)

    active = ha_helpers.runner(active_host)
    observer = ha_helpers.runner(observer_host)

    active_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_SERVER]
    observer_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_AUX]

    ha_helpers.server_install(active, active_ip, "62181")
    ha_helpers.server_install(observer, observer_ip, "62182")

    ha_helpers.ha(active, "group", "create", ha_helpers.GROUP_ID, ha_helpers.GROUP_TAG)

    ha_helpers.ha(observer, "peer", "self", "observer")
    ha_helpers.ha(observer, "peer", "add", "active", ha_helpers.control_endpoint(active_ip), ha_helpers.SECRET, "--allow-insecure")

    with _sessions.xp2p_run_session(observer_host, "server", helpers.INSTALL_ROOT, helpers.SERVER_CONFIG_DIR_NAME):
        ha_helpers.ha(active, "peer", "self", "active")
        ha_helpers.ha(active, "peer", "add", "lost", "https://10.62.10.250:62022", ha_helpers.SECRET, "--allow-insecure")
        ha_helpers.ha(
            active,
            "peer",
            "add",
            "observer",
            ha_helpers.control_endpoint(observer_ip),
            ha_helpers.SECRET,
            "--allow-insecure",
            "--non-voting",
        )
        result = ha_helpers.ha(
            active,
            "member",
            "add",
            "blocked",
            "blocked-endpoint",
            active_ip,
            "62181",
            "trojan-tls",
            check=False,
        )
        assert result.rc != 0
        assert "quorum is unavailable" in f"{result.stdout}\n{result.stderr}".lower()
        assert ha_helpers.generation_number(active_host) == 1


@pytest.mark.host
@pytest.mark.linux
def test_ha_client_switches_to_backup_after_primary_loss(linux_host_factory):
    client_host = linux_host_factory(linux_env.DEFAULT_CLIENT)
    primary_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    backup_host = linux_host_factory(linux_env.DEFAULT_AUX)

    client = ha_helpers.runner(client_host)
    primary = ha_helpers.runner(primary_host)
    backup = ha_helpers.runner(backup_host)

    primary_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_SERVER]
    backup_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_AUX]

    _clear_isolation(primary_host, "62191")
    _clear_isolation(backup_host, "62192")
    primary_interface = _interface_for_host_ip(primary_host, primary_ip)
    backup_interface = _interface_for_host_ip(backup_host, backup_ip)
    _remove_redirect_target(primary_host, primary_interface)
    _remove_redirect_target(backup_host, backup_interface)
    _remove_client_blackhole(client_host)

    primary_install = primary(
        "server", "install", "--json", "--path", helpers.INSTALL_ROOT.as_posix(), "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME, "--port", "62191", "--host", primary_ip, "--force", check=True,
    )
    backup_install = backup(
        "server", "install", "--json", "--path", helpers.INSTALL_ROOT.as_posix(), "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME, "--port", "62192", "--host", backup_ip, "--force", check=True,
    )
    primary_credential = helpers.parse_json_credential(primary_install.stdout or "")
    backup_credential = helpers.parse_json_credential(backup_install.stdout or "")
    backup_tls_name, backup_tls_pin = ha_helpers.link_tls_metadata(backup_credential["link"])
    backup(
        "server", "user", "add", "--json",
        "--path", helpers.INSTALL_ROOT.as_posix(),
        "--config-dir", helpers.SERVER_CONFIG_DIR_NAME,
        "--id", primary_credential["user"],
        "--password", primary_credential["password"],
        "--host", backup_ip,
        check=True,
    )

    ha_helpers.client_install_link(client, primary_credential["link"])
    primary_tag = helpers.expected_proxy_tag(primary_ip)
    backup_tag = "ha-backup-endpoint"
    ha_helpers.assert_client_has_no_endpoint_groups(client_host)

    ha_helpers.ha(primary, "group", "create", ha_helpers.CLIENT_GROUP_ID, ha_helpers.CLIENT_GROUP_TAG)
    ha_helpers.ha(primary, "member", "add", "primary", primary_tag, primary_ip, "62191", "trojan-tls")
    ha_helpers.ha(
        primary,
        "member",
        "add",
        "backup",
        backup_tag,
        backup_ip,
        "62192",
        "trojan-tls",
        "--tls-server-name",
        backup_tls_name,
        "--tls-pin",
        backup_tls_pin,
    )
    ha_helpers.assert_generation_member(primary_host, "primary")
    ha_helpers.assert_generation_member(primary_host, "backup")
    ha_helpers.ha(primary, "channel", "create", "client-portal", HA_REVERSE_TAG, HA_REVERSE_TAG)
    ha_helpers.ha(primary, "redirect", "add", "client-portal", "--cidr", HA_SERVER_REDIRECT_CIDR)

    primary_isolated = False
    backup_isolated = False
    client_api_rewritten = False
    client_live_xray_before_api_rewrite = ""
    try:
        _add_redirect_target(primary_host, primary_interface)
        _add_redirect_target(backup_host, backup_interface)
        _add_client_blackhole(client_host)
        ha_helpers.ha(primary, "peer", "self", "primary")
        ha_helpers.ha(primary, "peer", "add", "backup", ha_helpers.control_endpoint(backup_ip), ha_helpers.SECRET, "--allow-insecure")
        ha_helpers.ha(backup, "peer", "self", "backup")
        ha_helpers.ha(backup, "peer", "add", "primary", ha_helpers.control_endpoint(primary_ip), ha_helpers.SECRET, "--allow-insecure")

        _start_service_debug(backup_host, backup, "server")
        ha_helpers.ha(primary, "sync")
        ha_helpers.assert_generation_member(backup_host, "primary")
        ha_helpers.assert_generation_member(backup_host, "backup")
        runtime.wait_for_apply_clear(backup_host)
        try:
            ha_helpers.assert_server_live_subscription_topology(backup_host, ha_helpers.CLIENT_GROUP_TAG)
            _assert_ha_server_resources(backup_host, backup, HA_REVERSE_TAG, HA_SERVER_REDIRECT_CIDR)
        except AssertionError as exc:
            pytest.fail(f"{exc}\n\n{ha_helpers.server_subscription_debug(backup_host)}")

        _start_service_debug(primary_host, primary, "server")
        try:
            ha_helpers.assert_server_live_subscription_topology(primary_host, ha_helpers.CLIENT_GROUP_TAG)
            _assert_ha_server_resources(primary_host, primary, HA_REVERSE_TAG, HA_SERVER_REDIRECT_CIDR)
        except AssertionError as exc:
            pytest.fail(f"{exc}\n\n{ha_helpers.server_subscription_debug(primary_host)}")
        _start_client_service_for_subscription(client_host, client)

        try:
            ha_helpers.wait_for_group_active(client, primary_tag, timeout_seconds=75.0)
        except AssertionError as exc:
            pytest.fail(
                f"{exc}\n\n"
                f"CLIENT:\n{ha_helpers.client_endpoint_group_debug(client_host)}\n\n"
                f"PRIMARY SERVER:\n{ha_helpers.server_subscription_debug(primary_host)}\n\n"
                f"PRIMARY CONTROL FROM CLIENT:\n{ha_helpers.server_control_probe(client_host, primary_ip)}"
            )
        runtime.wait_for_apply_clear(client_host)
        ha_helpers.assert_tunnel_ping(client, primary_ip, primary_tag)
        _assert_selector_committed(client_host, primary_tag)

        subscription_apply_count = _client_subscription_apply_count(client_host)
        ha_helpers.ha(primary, "member", "reprioritize", "backup", "5")
        runtime.wait_for_apply_clear(primary_host)
        ha_helpers.ha(primary, "sync")
        runtime.wait_for_apply_clear(backup_host)
        _wait_for_client_subscription_apply_count(client_host, subscription_apply_count)
        ha_helpers.wait_for_group_active(client, primary_tag, timeout_seconds=20.0)
        ha_helpers.assert_tunnel_ping(client, primary_ip, primary_tag)
        _assert_selector_committed(client_host, primary_tag)

        _isolate_server(primary_host, "62191")
        primary_isolated = True
        time.sleep(1.0)
        _clear_isolation(primary_host, "62191")
        primary_isolated = False
        ha_helpers.wait_for_group_active(client, primary_tag, timeout_seconds=20.0)
        ha_helpers.assert_tunnel_ping(client, primary_ip, primary_tag)

        client(
            "client", "redirect", "add", "--path", helpers.INSTALL_ROOT.as_posix(), "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME, "--cidr", HA_REDIRECT_CIDR, "--tag", ha_helpers.CLIENT_GROUP_TAG, check=True,
        )
        routing_with_redirect = helpers.render_xray(client_host, client, "client", desired=False)
        helpers.assert_redirect_rule(routing_with_redirect, HA_REDIRECT_CIDR, ha_helpers.CLIENT_GROUP_TAG)
        _assert_redirected_tunnel_ping(client)

        client_live_xray_before_api_rewrite = helpers.read_text(client_host, helpers.CLIENT_LIVE_DIR / "xray.json")
        _rewrite_client_live_api_listen(client_host, "127.0.0.1:1")
        client_api_rewritten = True
        _isolate_server(primary_host, "62191")
        primary_isolated = True
        time.sleep(8.0)
        _wait_for_client_runtime_switch_apply_failure(client_host)
        _assert_selector_committed(client_host, primary_tag)
        _restore_client_live_xray(client_host, client_live_xray_before_api_rewrite)
        client_api_rewritten = False
        _clear_isolation(primary_host, "62191")
        primary_isolated = False
        ha_helpers.wait_for_group_active(client, primary_tag, timeout_seconds=20.0)
        ha_helpers.assert_tunnel_ping(client, primary_ip, primary_tag)

        runtime.stop_service(primary, "server")
        runtime.wait_for_service(primary_host, "server", active=False)
        primary_host.run("sudo -n pkill -f '/etc/xp2p/bin/[x]ray' >/dev/null 2>&1 || true")
        primary_host.run("sudo -n pkill -f '[x]p2p server' >/dev/null 2>&1 || true")
        _isolate_server(primary_host, "62191")
        primary_isolated = True

        try:
            ha_helpers.wait_for_group_active(client, backup_tag, timeout_seconds=75.0)
        except AssertionError as exc:
            pytest.fail(f"{exc}\n\n{ha_helpers.client_endpoint_group_debug(client_host)}")
        ha_helpers.assert_tunnel_ping(client, backup_ip, backup_tag)
        _assert_selector_committed(client_host, backup_tag)
        _assert_redirected_tunnel_ping(client)

        _clear_isolation(primary_host, "62191")
        primary_isolated = False
        _start_service_debug(primary_host, primary, "server")
        ha_helpers.wait_for_group_active(client, backup_tag, timeout_seconds=20.0)

        runtime.stop_service(backup, "server")
        runtime.wait_for_service(backup_host, "server", active=False)
        backup_host.run("sudo -n pkill -f '/etc/xp2p/bin/[x]ray' >/dev/null 2>&1 || true")
        backup_host.run("sudo -n pkill -f '[x]p2p server' >/dev/null 2>&1 || true")
        _isolate_server(backup_host, "62192")
        backup_isolated = True
        try:
            ha_helpers.wait_for_group_active(client, primary_tag, timeout_seconds=75.0)
        except AssertionError as exc:
            pytest.fail(f"{exc}\n\n{ha_helpers.client_endpoint_group_debug(client_host)}")
        ha_helpers.assert_tunnel_ping(client, primary_ip, primary_tag)
        _assert_selector_committed(client_host, primary_tag)
        _assert_redirected_tunnel_ping(client)

        client(
            "client", "redirect", "remove", "--path", helpers.INSTALL_ROOT.as_posix(), "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME, "--cidr", HA_REDIRECT_CIDR, "--tag", ha_helpers.CLIENT_GROUP_TAG, check=True,
        )
        routing_after_remove = helpers.render_xray(client_host, client, "client", desired=False)
        helpers.assert_no_redirect_rule(routing_after_remove, HA_REDIRECT_CIDR, ha_helpers.CLIENT_GROUP_TAG)
    finally:
        if client_api_rewritten and client_live_xray_before_api_rewrite:
            _restore_client_live_xray(client_host, client_live_xray_before_api_rewrite)
        if primary_isolated:
            _clear_isolation(primary_host, "62191")
        if backup_isolated:
            _clear_isolation(backup_host, "62192")
        _remove_redirect_target(primary_host, primary_interface)
        _remove_redirect_target(backup_host, backup_interface)
        _remove_client_blackhole(client_host)
        runtime.stop_service(client, "client")
        runtime.stop_service(primary, "server")
        runtime.stop_service(backup, "server")


def _start_client_service_for_subscription(host, run) -> None:
    _start_service_debug(host, run, "client")


def _assert_ha_server_resources(host, run, reverse_tag: str, redirect_cidr: str) -> None:
    state = helpers.read_pending_server_config(host)
    helpers.assert_server_reverse_state(state, reverse_tag)
    helpers.assert_server_redirect_state(state, redirect_cidr, reverse_tag)
    live = helpers.render_xray(host, run, "server", desired=False)
    helpers.assert_server_reverse_routing(live, reverse_tag)
    helpers.assert_server_redirect_rule(live, redirect_cidr, reverse_tag)


def _assert_selector_committed(host, expected_tag: str) -> None:
    group_id = ha_helpers.CLIENT_GROUP_ID.lower()
    state_path = helpers.CLIENT_LIVE_DIR / "endpoint-selector.json"
    journal_path = helpers.CLIENT_LIVE_DIR / "endpoint-selector.journal.json"
    deadline = time.time() + 10.0
    last_snapshot = None
    while time.time() < deadline:
        state_before = helpers.read_json(host, state_path)
        journal = helpers.read_json(host, journal_path)
        state_after = helpers.read_json(host, state_path)
        last_snapshot = (state_before, journal, state_after)
        revision = state_before.get("revision")
        if revision != state_after.get("revision") or revision != journal.get("revision"):
            time.sleep(0.2)
            continue
        selected = ((state_after.get("groups") or {}).get(group_id) or {}).get("active_tag")
        journal_selected = ((journal.get("groups") or {}).get(group_id) or {}).get("active_tag")
        if selected == expected_tag and journal_selected == expected_tag:
            return
        time.sleep(0.2)
    raise AssertionError(
        "selector did not expose a stable committed snapshot: "
        f"state_before={last_snapshot[0]}, journal={last_snapshot[1]}, "
        f"state_after={last_snapshot[2]}, expected={expected_tag}"
    )


def _client_subscription_apply_count(host) -> int:
    log_path = helpers.LOG_ROOT / "client" / "service.log"
    if not helpers.path_exists(host, log_path):
        return 0
    return helpers.read_text(host, log_path).count("subscription applied")


def _wait_for_client_subscription_apply_count(host, previous_count: int) -> int:
    deadline = time.time() + 75.0
    last_count = previous_count
    while time.time() < deadline:
        last_count = _client_subscription_apply_count(host)
        if last_count > previous_count:
            return last_count
        time.sleep(2.0)
    raise AssertionError(f"client did not apply refreshed HA subscription; before={previous_count}, last={last_count}")


def _rewrite_client_live_api_listen(host, listen: str) -> None:
    path = helpers.CLIENT_LIVE_DIR / "xray.json"
    data = helpers.read_json(host, path)
    data.setdefault("api", {})["listen"] = listen
    helpers.write_text(host, path, json.dumps(data, indent=2) + "\n")


def _restore_client_live_xray(host, content: str) -> None:
    helpers.write_text(host, helpers.CLIENT_LIVE_DIR / "xray.json", content)


def _wait_for_client_runtime_switch_apply_failure(host) -> None:
    deadline = time.time() + 35.0
    log_path = helpers.LOG_ROOT / "client" / "service.log"
    last_log = ""
    while time.time() < deadline:
        if helpers.path_exists(host, log_path):
            last_log = helpers.read_text(host, log_path)
            if "endpoint group health update failed" in last_log and (
                "runtime apply" in last_log or "connect" in last_log or "connection refused" in last_log
            ):
                return
        time.sleep(1.5)
    raise AssertionError(f"client runtime switch apply failure was not logged.\n{last_log[-4000:]}")


def _start_service_debug(host, run, role: str) -> None:
    host.run("sudo -n systemctl daemon-reload >/dev/null 2>&1 || true")
    run(role, "service", "stop")
    host.run("sudo -n pkill -f '/etc/xp2p/bin/[x]ray' >/dev/null 2>&1 || true")
    run("--log-level", "debug", role, "service", "start", check=True)
    runtime.wait_for_service(host, role, active=True)
    runtime.wait_for_live_xray(host, role)


def _isolate_server(host, tunnel_port: str) -> None:
    for port in ("62022", tunnel_port):
        host.run(f"sudo -n iptables -I INPUT -p tcp --dport {port} -j REJECT")


def _clear_isolation(host, tunnel_port: str) -> None:
    for port in ("62022", tunnel_port):
        host.run(
            "while sudo -n iptables -D INPUT -p tcp --dport "
            f"{port} -j REJECT >/dev/null 2>&1; do :; done"
        )


def _interface_for_host_ip(host, host_ip: str) -> str:
    result = host.run(f"ip route get {host_ip} | awk '/dev/ {{for (i = 1; i <= NF; i++) if ($i == \"dev\") {{print $(i + 1); exit}}}}'")
    interface = (result.stdout or "").strip()
    assert interface, result.stderr
    return interface


def _add_redirect_target(host, interface: str) -> None:
    host.run(f"sudo -n ip addr add {HA_REDIRECT_CIDR} dev {interface} 2>/dev/null || true")


def _remove_redirect_target(host, interface: str) -> None:
    host.run(f"sudo -n ip addr del {HA_REDIRECT_CIDR} dev {interface} >/dev/null 2>&1 || true")


def _add_client_blackhole(host) -> None:
    host.run(f"sudo -n ip route replace blackhole {HA_REDIRECT_CIDR}")


def _remove_client_blackhole(host) -> None:
    host.run(f"sudo -n ip route del {HA_REDIRECT_CIDR} >/dev/null 2>&1 || true")


def _assert_redirected_tunnel_ping(run) -> None:
    result = run("ping", HA_REDIRECT_IP, "--tunnel", "--count", "3", check=False)
    output = f"{result.stdout}\n{result.stderr}".lower()
    assert result.rc == 0 and "0% loss" in output, output
