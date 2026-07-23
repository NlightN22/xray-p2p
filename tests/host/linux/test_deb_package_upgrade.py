from __future__ import annotations

import pytest

from tests.host.linux import _credential_migration_upgrade as upgrade


pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.serial]

OVERRIDE_DIR = "/etc/systemd/system/xp2p-server.service.d"
OVERRIDE_PATH = f"{OVERRIDE_DIR}/upgrade-test.conf"
STATE_ROOT = "/var/lib/xp2p"
STATE_PATH = f"{STATE_ROOT}/deb-upgrade-services"


def cleanup_upgrade_override(host) -> None:
    host.run("sudo -n systemctl stop xp2p-server.service")
    host.run(f"sudo -n rm -f {OVERRIDE_PATH}")
    host.run(f"sudo -n rmdir {OVERRIDE_DIR} 2>/dev/null || true")
    host.run(f"sudo -n rm -f {STATE_PATH}")
    host.run("sudo -n systemctl daemon-reload")


def seed_legacy_upgrade_state(host) -> None:
    host.run(f"sudo -n install -d -m 0755 {STATE_ROOT}")
    host.run(
        "printf '%s\\n' "
        "'xp2p-client.service 1 0' "
        "'xp2p-server.service 1 1' | "
        f"sudo -n tee {STATE_PATH} >/dev/null"
    )
    host.run(f"sudo -n chmod 0600 {STATE_PATH}")


def test_deb_upgrade_preserves_enabled_running_service(server_host):
    cleanup_upgrade_override(server_host)
    previous = upgrade.ensure_previous_release_deb("0.2.6")
    candidate = upgrade.current_candidate_deb()
    upgrade.install_deb(server_host, previous)
    try:
        server_host.run(f"sudo -n mkdir -p {OVERRIDE_DIR}")
        server_host.run(
            "printf '%s\\n' '[Service]' 'ExecStart=' "
            "'ExecStart=/bin/sleep infinity' | "
            f"sudo -n tee {OVERRIDE_PATH} >/dev/null"
        )
        server_host.run("sudo -n systemctl daemon-reload")
        server_host.run("sudo -n systemctl enable --now xp2p-server.service")
        assert server_host.service("xp2p-server").is_enabled
        assert server_host.service("xp2p-server").is_running

        seed_legacy_upgrade_state(server_host)
        upgrade.install_deb(server_host, candidate)

        assert server_host.service("xp2p-server").is_enabled
        assert server_host.service("xp2p-server").is_running
    finally:
        cleanup_upgrade_override(server_host)


def test_deb_upgrade_from_current_preserves_service_state(server_host):
    cleanup_upgrade_override(server_host)
    candidate = upgrade.current_candidate_deb()
    upgrade.install_deb(server_host, candidate)
    try:
        server_host.run(f"sudo -n mkdir -p {OVERRIDE_DIR}")
        server_host.run(
            "printf '%s\\n' '[Service]' 'ExecStart=' "
            "'ExecStart=/bin/sleep infinity' | "
            f"sudo -n tee {OVERRIDE_PATH} >/dev/null"
        )
        server_host.run("sudo -n systemctl daemon-reload")
        server_host.run("sudo -n systemctl disable --now xp2p-client.service")
        server_host.run("sudo -n systemctl enable --now xp2p-server.service")

        upgrade.install_deb(server_host, candidate)

        assert not server_host.service("xp2p-client").is_enabled
        assert not server_host.service("xp2p-client").is_running
        assert server_host.service("xp2p-server").is_enabled
        assert server_host.service("xp2p-server").is_running
    finally:
        cleanup_upgrade_override(server_host)
