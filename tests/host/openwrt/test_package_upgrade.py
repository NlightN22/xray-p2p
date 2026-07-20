from __future__ import annotations

from pathlib import PurePosixPath

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.destructive
def test_openwrt_upgrade_preserves_running_client_service(
    openwrt_host, openwrt_ipk_target, xp2p_openwrt_ipk
):
    previous_ipk = openwrt_env.ensure_previous_release_ipk(
        "0.2.7", openwrt_ipk_target
    )
    for machine in openwrt_env.OPENWRT_MACHINES:
        openwrt_env.sync_build_output(machine)
    openwrt_env.install_ipk_on_host(openwrt_host, previous_ipk, force=True)
    staged = openwrt_env.stage_ipk_on_guest(
        openwrt_host,
        xp2p_openwrt_ipk,
        PurePosixPath("/tmp/xp2p-upgrade-candidate.ipk"),
    )
    runner = lambda *cmd: openwrt_env.run_xp2p(openwrt_host, *cmd)
    helpers.cleanup_client_install(openwrt_host, runner)

    install = runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--host",
        "upgrade-recovery.example.com",
        "--user",
        "upgrade-user",
        "--password",
        "upgrade-secret",
        "--force",
    )
    assert install.rc == 0, install.stderr
    start = runner("client", "service", "start")
    assert start.rc == 0, start.stderr
    helpers.wait_for_service_state(openwrt_host, "client", running=True)

    try:
        previous_hash = openwrt_host.run("sha256sum /usr/bin/xp2p").stdout.split()[0]
        upgraded = openwrt_host.run(
            f"opkg install --force-reinstall {staged}", timeout=120
        )
        assert upgraded.rc == 0, upgraded.stderr
        helpers.wait_for_service_state(openwrt_host, "client", running=True)

        candidate_hash = openwrt_host.run("sha256sum /usr/bin/xp2p").stdout.split()[0]
        assert candidate_hash != previous_hash
        assert helpers.path_exists(
            openwrt_host, helpers.INSTALL_ROOT / "bin" / "xray"
        )
    finally:
        runner("client", "service", "stop")
        helpers.cleanup_client_install(openwrt_host, runner)
