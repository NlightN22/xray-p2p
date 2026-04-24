from __future__ import annotations

import pytest

pytestmark = [pytest.mark.host, pytest.mark.linux]


def test_client_deploy_mode_tun_fails_when_tun_device_missing(client_host, xp2p_client_runner):
    original = "/dev/net/tun"
    backup = "/dev/net/tun.xp2p-testbak"

    if client_host.run(f"sudo -n test -e {original}").rc != 0:
        pytest.skip(f"{original} missing on test host")

    rename_out = client_host.run(f"sudo -n sh -c 'mv {original} {backup}'")
    if rename_out.rc != 0:
        pytest.skip(f"unable to hide {original}: {rename_out.stderr}")

    try:
        result = xp2p_client_runner(
            "client",
            "deploy",
            "--host",
            "127.0.0.1",
            "--mode",
            "tun",
            check=False,
        )
        combined = (result.stdout or "") + "\n" + (result.stderr or "")
        assert result.rc != 0, "expected deploy to fail when tun device is missing"
        assert "/dev/net/tun" in combined, f"expected error to mention {original}:\n{combined}"
        assert "tun is unavailable" in combined.lower(), f"expected error to mention tun preflight:\n{combined}"
    finally:
        client_host.run(f"sudo -n sh -c 'if [ -e {backup} ]; then mv {backup} {original}; fi'")
