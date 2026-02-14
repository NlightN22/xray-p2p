from __future__ import annotations

import pytest
from testinfra.host import Host

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

pytestmark = [pytest.mark.host, pytest.mark.linux]

DEPLOY_PORT = "62210"
TROJAN_PORT = "58611"


def _runner(host: Host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_client_deploy_rejects_duplicate_endpoint(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)

    runner = _runner(openwrt_host)
    host_ip = helpers.detect_primary_ipv4(openwrt_host)
    try:
        helpers.cleanup_client_install(openwrt_host, runner)

        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            host_ip,
            "--port",
            TROJAN_PORT,
            "--user",
            "dup-deploy@example.com",
            "--password",
            "dup-deploy-pass",
            check=True,
        )

        result = runner(
            "client",
            "deploy",
            "--host",
            host_ip,
            "--port",
            DEPLOY_PORT,
            "--user",
            "dup-deploy@example.com",
            "--password",
            "dup-deploy-pass",
            "--trojan-port",
            TROJAN_PORT,
            check=False,
        )
        assert result.rc != 0
        combined = f"{result.stdout}\n{result.stderr}".lower()
        assert f"endpoint {host_ip}".lower() in combined and "already exists" in combined
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
