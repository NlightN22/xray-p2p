from __future__ import annotations

import re

import pytest

from tests.host.openwrt import env as openwrt_env


@pytest.fixture(scope="module")
def xp2p_installed(openwrt_client_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_client_host, xp2p_openwrt_ipk, force=True)
    return xp2p_openwrt_ipk


def test_nat_redirect_print_only_no_changes(openwrt_client_host, xp2p_installed):
    cidr = "10.123.0.0/24"
    port = "54321"
    snippet = "/tmp/xp2p-nat-test.nft"
    entry_dir = "/tmp/xp2p-nat-test.d"

    add = openwrt_client_host.run(
        f"/usr/bin/xp2p nat-redirect add --cidr {cidr} --port {port} "
        f"--print-only --snippet {snippet} --entry-dir {entry_dir}"
    )
    assert add.rc == 0, f"add command failed: {add.stderr}"
    stdout = add.stdout or ""
    assert snippet in stdout
    assert entry_dir in stdout
    assert re.search(rf"iptables .* -d {re.escape(cidr)} .* --to-ports {port}", stdout)

    list_result = openwrt_client_host.run("/usr/bin/xp2p nat-redirect list")
    assert list_result.rc == 0
    assert "No transparent redirects configured." in (list_result.stdout or "")
