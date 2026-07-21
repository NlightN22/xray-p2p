from __future__ import annotations

import json
from pathlib import PurePosixPath

import pytest

from tests.host import _credential_migration as credentials
from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env


pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.destructive]

SERVER_HOST = "credential-upgrade.example.test"
SERVER_PORT = "62310"
USER = "upgrade-client@example.test"
OLD_CREDENTIAL = "550e8400-e29b-41d4-a716-446655440020"


def test_openwrt_upgrade_migrates_previous_release_client_credential(
    openwrt_server_host,
    openwrt_client_host,
    openwrt_ipk_target,
    xp2p_openwrt_ipk,
):
    previous_ipk = openwrt_env.ensure_previous_release_ipk("0.2.7", openwrt_ipk_target)
    for machine in openwrt_env.OPENWRT_MACHINES:
        openwrt_env.sync_build_output(machine)

    openwrt_env.install_ipk_on_host(openwrt_server_host, previous_ipk, force=True)
    openwrt_env.install_ipk_on_host(openwrt_client_host, previous_ipk, force=True)
    server_candidate = openwrt_env.stage_ipk_on_guest(
        openwrt_server_host,
        xp2p_openwrt_ipk,
        PurePosixPath("/tmp/xp2p-server-upgrade-candidate.ipk"),
    )
    client_candidate = openwrt_env.stage_ipk_on_guest(
        openwrt_client_host,
        xp2p_openwrt_ipk,
        PurePosixPath("/tmp/xp2p-client-upgrade-candidate.ipk"),
    )
    server_runner = _runner(openwrt_server_host)
    client_runner = _runner(openwrt_client_host)
    server_ip = _detect_host_ipv4(openwrt_server_host)
    _add_hosts_entry(openwrt_client_host, server_ip)

    try:
        server_runner(
            "server", "install", "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME, "--host", SERVER_HOST,
            "--port", SERVER_PORT, "--force", check=True,
        )
        added = server_runner(
            "server", "user", "add", "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME, "--id", USER,
            "--password", OLD_CREDENTIAL, "--host", SERVER_HOST, check=True,
        )
        link = credentials.connection_link(added.stdout or "")
        client_runner(
            "client", "install", "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.CLIENT_CONFIG_DIR_NAME, "--link", link,
            "--mode", "proxy", check=True,
        )
        server_runner("server", "service", "start", check=True)
        client_runner("client", "service", "start", check=True)
        helpers.wait_for_service_state(openwrt_server_host, "server", expected_active=True)
        helpers.wait_for_service_state(openwrt_client_host, "client", expected_active=True)
        _wait_for_client_convergence(openwrt_client_host, OLD_CREDENTIAL)
        _assert_tunnel_ping(client_runner)
        client_heartbeat_before = _wait_for_heartbeat_observation(
            openwrt_client_host, helpers.CLIENT_HEARTBEAT_STATE_FILE
        )
        server_heartbeat_before = _wait_for_heartbeat_observation(
            openwrt_server_host, helpers.SERVER_HEARTBEAT_STATE_FILE
        )

        client_runner("client", "service", "stop", check=True)
        server_runner("server", "user", "rotate", USER, check=True)
        rotated = credentials.server_user(
            helpers.read_pending_server_config(openwrt_server_host), USER
        )
        active = rotated["active_credential"]
        assert active != OLD_CREDENTIAL
        assert rotated["previous_credential_for_rotation"] == OLD_CREDENTIAL
        server_desired_digest = _file_digest(openwrt_server_host, helpers.SERVER_CONFIG_FILE)
        client_desired_digest = _file_digest(openwrt_client_host, helpers.CLIENT_CONFIG_FILE)

        _upgrade(openwrt_server_host, server_candidate)
        _upgrade(openwrt_client_host, client_candidate)
        assert _file_digest(openwrt_server_host, helpers.SERVER_CONFIG_FILE) == server_desired_digest
        assert _file_digest(openwrt_client_host, helpers.CLIENT_CONFIG_FILE) == client_desired_digest
        _assert_upgrade_archive(openwrt_server_host, "state-heartbeat-server.json")
        _assert_upgrade_archive(openwrt_client_host, "state-heartbeat-client.json")
        server_runner("server", "service", "start", check=True)
        client_runner("client", "service", "start", check=True)
        helpers.wait_for_service_state(openwrt_server_host, "server", expected_active=True)
        helpers.wait_for_service_state(openwrt_client_host, "client", expected_active=True)

        _wait_for_client_convergence(openwrt_client_host, active)
        _assert_tunnel_ping(client_runner)
        _wait_for_fresh_heartbeat(
            openwrt_client_host,
            helpers.CLIENT_HEARTBEAT_STATE_FILE,
            client_heartbeat_before,
        )
        _wait_for_fresh_heartbeat(
            openwrt_server_host,
            helpers.SERVER_HEARTBEAT_STATE_FILE,
            server_heartbeat_before,
        )
        credentials.wait_until(
            lambda: credentials.server_user(
                helpers.read_pending_server_config(openwrt_server_host), USER
            ),
            lambda state: not state.get("previous_credential_for_rotation"),
            timeout=90.0,
            description="OpenWrt server credential acknowledgement",
        )
        _assert_previous_rejected_by_server_xray(
            openwrt_client_host, client_runner, active
        )

        client_runner("client", "service", "stop", check=True)
        server_runner("server", "service", "stop", check=True)
        server_runner("server", "service", "start", check=True)
        client_runner("client", "service", "start", check=True)
        helpers.wait_for_service_state(
            openwrt_server_host, "server", expected_active=True
        )
        helpers.wait_for_service_state(
            openwrt_client_host, "client", expected_active=True
        )
        _wait_for_client_convergence(openwrt_client_host, active)
        _assert_tunnel_ping(client_runner)
        assert not credentials.server_user(
            helpers.read_pending_server_config(openwrt_server_host), USER
        ).get("previous_credential_for_rotation")
        _assert_previous_rejected_by_server_xray(
            openwrt_client_host, client_runner, active
        )
    finally:
        client_runner("client", "service", "stop")
        server_runner("server", "service", "stop")
        helpers.cleanup_client_install(openwrt_client_host, client_runner)
        helpers.cleanup_server_install(openwrt_server_host, server_runner)
        _remove_hosts_entry(openwrt_client_host)


def _runner(host):
    def run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
        if check:
            assert result.rc == 0, f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        return result

    return run


def _upgrade(host, candidate: PurePosixPath) -> None:
    before = host.run("sha256sum /usr/bin/xp2p").stdout.split()[0]
    result = host.run(f"opkg install --force-reinstall {candidate}", timeout=120)
    assert result.rc == 0, result.stderr
    after = host.run("sha256sum /usr/bin/xp2p").stdout.split()[0]
    assert after != before


def _heartbeat_observation(host, path: PurePosixPath) -> tuple[str, int] | None:
    result = host.run(f"cat {path}")
    if result.rc != 0:
        return None
    entries = (json.loads(result.stdout or "{}").get("entries") or {}).values()
    observations = [
        (str(entry.get("last_seen") or ""), int(entry.get("samples") or 0))
        for entry in entries
        if entry.get("last_seen")
    ]
    return max(observations) if observations else None


def _wait_for_heartbeat_observation(host, path: PurePosixPath) -> tuple[str, int]:
    return credentials.wait_until(
        lambda: _heartbeat_observation(host, path),
        bool,
        timeout=90.0,
        description=f"heartbeat observation in {path}",
    )


def _wait_for_fresh_heartbeat(
    host, path: PurePosixPath, baseline: tuple[str, int]
) -> None:
    credentials.wait_until(
        lambda: _heartbeat_observation(host, path),
        lambda current: current is not None and current != baseline,
        timeout=90.0,
        description=f"fresh heartbeat observation in {path}",
    )


def _assert_upgrade_archive(host, heartbeat_name: str) -> None:
    archive = host.run("ls -1t /etc/xp2p/upgrade-archives/state-*.tar.gz | head -n 1")
    assert archive.rc == 0 and archive.stdout.strip(), archive.stderr
    listing = host.run(f"tar -tzf {archive.stdout.strip()}")
    assert listing.rc == 0, listing.stderr
    names = set(line.removeprefix("./") for line in listing.stdout.splitlines())
    assert heartbeat_name in names, listing.stdout
    assert any(name.startswith(".state/live/") for name in names), listing.stdout
    assert any(name.startswith(".state/lkg/") for name in names), listing.stdout


def _file_digest(host, path: PurePosixPath) -> str:
    result = host.run(f"sha256sum {path}")
    assert result.rc == 0, result.stderr
    return result.stdout.split()[0]


def _assert_client_converged(host, expected: str) -> None:
    credentials.assert_client_persisted_credential_converged(
        helpers.read_pending_client_config(host),
        helpers.read_live_json(host, helpers.CLIENT_LIVE_DIR / "runtime.json"),
        helpers.read_live_json(host, helpers.CLIENT_LIVE_DIR / "xray.json"),
        USER,
        expected,
    )


def _wait_for_client_convergence(host, expected: str) -> None:
    def converged():
        try:
            _assert_client_converged(host, expected)
            return True
        except (AssertionError, RuntimeError):
            return False

    credentials.wait_until(
        converged,
        bool,
        timeout=120.0,
        description="OpenWrt client credential migration",
    )


def _assert_tunnel_ping(runner) -> None:
    def probe():
        result = runner("ping", SERVER_HOST, "-T", "--count", "2")
        return result.rc, result.stdout or "", result.stderr or ""

    credentials.wait_until(
        probe,
        lambda result: result[0] == 0 and "0% loss" in result[1].lower(),
        timeout=60.0,
        description="OpenWrt tunnel after credential migration",
    )


def _assert_previous_rejected_by_server_xray(host, runner, active: str) -> None:
    config = helpers.read_live_json(host, helpers.CLIENT_LIVE_DIR / "xray.json")
    socks_port = 0
    for inbound in config.get("inbounds") or []:
        if inbound.get("protocol") == "socks":
            inbound["port"] = 52980
            socks_port = 52980
        elif isinstance(inbound.get("port"), int):
            inbound["port"] += 1000
    assert socks_port, "Client SOCKS inbound is missing"
    config.setdefault("api", {})["listen"] = "127.0.0.1:52981"
    replaced = False
    for outbound in config.get("outbounds") or []:
        for server in outbound.get("settings", {}).get("servers") or []:
            if server.get("email") == USER and server.get("password") == active:
                server["password"] = OLD_CREDENTIAL
                replaced = True
    assert replaced, "Active credential is missing from client Live Xray artifact"

    config_path = PurePosixPath("/tmp/xp2p-previous-credential-xray.json")
    pid_path = PurePosixPath("/tmp/xp2p-previous-credential-xray.pid")
    helpers.write_text(host, config_path, json.dumps(config) + "\n")
    start = host.run(
        f"/etc/xp2p/bin/xray run -c {config_path} >/tmp/xp2p-previous-xray.log 2>&1 "
        f"& echo $! > {pid_path}"
    )
    assert start.rc == 0, start.stderr
    try:
        credentials.wait_until(
            lambda: host.run(f"netstat -ltn 2>/dev/null | grep -q ':{socks_port} '").rc,
            lambda rc: rc == 0,
            timeout=15.0,
            description="temporary previous-credential Xray SOCKS listener",
        )
        rejected = runner(
            "ping", SERVER_HOST, "-T", f"127.0.0.1:{socks_port}",
            "--count", "1", "--timeout", "5",
        )
        assert rejected.rc != 0, "Server Xray accepted the previous Trojan credential"
    finally:
        host.run(f"kill $(cat {pid_path}) >/dev/null 2>&1 || true")
        host.run(f"rm -f {config_path} {pid_path} /tmp/xp2p-previous-xray.log")


def _add_hosts_entry(host, address: str) -> None:
    _remove_hosts_entry(host)
    host.run(
        f"printf '%s %s # xp2p-credential-migration\\n' '{address}' '{SERVER_HOST}' >> /etc/hosts"
    )


def _remove_hosts_entry(host) -> None:
    host.run("sed -i '/# xp2p-credential-migration$/d' /etc/hosts")


def _detect_host_ipv4(host) -> str:
    result = host.run("ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1")
    assert result.rc == 0, result.stderr
    addresses = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    assert addresses, "No global IPv4 address found"
    return next((address for address in addresses if not address.startswith("10.0.2.")), addresses[0])
