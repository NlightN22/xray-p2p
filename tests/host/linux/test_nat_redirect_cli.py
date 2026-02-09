from __future__ import annotations

import re

from tests.host.linux import env as linux_env


def test_nat_redirect_print_only_no_changes(client_host):
    cidr = "10.123.0.0/24"
    port = "54321"
    snippet = "/tmp/xp2p-nat-test.nft"
    entry_dir = "/tmp/xp2p-nat-test.d"
    env = {
        "XP2P_CLIENT_TUN_ENABLED": "false",
        "XP2P_SERVER_TUN_ENABLED": "false",
    }

    add = linux_env.run_xp2p_with_env(
        client_host,
        env,
        "nat-redirect",
        "add",
        "--cidr",
        cidr,
        "--port",
        port,
        "--print-only",
        "--snippet",
        snippet,
        "--entry-dir",
        entry_dir,
    )
    if add.rc != 0:
        raise AssertionError(
            "xp2p nat-redirect add failed.\n"
            f"STDOUT:\n{add.stdout}\nSTDERR:\n{add.stderr}"
        )
    stdout = add.stdout or ""
    assert snippet in stdout
    assert entry_dir in stdout
    assert re.search(rf"iptables .* -d {re.escape(cidr)} .* --to-ports {port}", stdout)

    list_result = linux_env.run_xp2p_with_env(client_host, env, "nat-redirect", "list")
    if list_result.rc != 0:
        raise AssertionError(
            "xp2p nat-redirect list failed.\n"
            f"STDOUT:\n{list_result.stdout}\nSTDERR:\n{list_result.stderr}"
        )
    assert "No transparent redirects configured." in (list_result.stdout or "")
