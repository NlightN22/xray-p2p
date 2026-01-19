from __future__ import annotations

import re


def test_nat_redirect_print_only_no_changes(xp2p_client_runner):
    cidr = "10.123.0.0/24"
    port = "54321"
    snippet = "/tmp/xp2p-nat-test.nft"
    entry_dir = "/tmp/xp2p-nat-test.d"

    add = xp2p_client_runner(
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
        check=True,
    )
    stdout = add.stdout or ""
    assert snippet in stdout
    assert entry_dir in stdout
    assert re.search(rf"iptables .* -d {re.escape(cidr)} .* --to-ports {port}", stdout)

    list_result = xp2p_client_runner("nat-redirect", "list", check=True)
    assert "No transparent redirects configured." in (list_result.stdout or "")
