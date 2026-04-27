from __future__ import annotations

import os

import pytest

from tests.host.openwrt import env as openwrt_env

RUN_ENV_VAR = "XP2P_RUN_OPENWRT_INSTALL_SCRIPT_TESTS"


@pytest.mark.host
@pytest.mark.linux
def test_install_script_from_pages(openwrt_host):
    flag = os.environ.get(RUN_ENV_VAR, "").strip().lower()
    if flag not in {"1", "true", "yes"}:
        pytest.skip(f"Set {RUN_ENV_VAR}=1 to run the OpenWrt install script integration test.")

    result = openwrt_env.run_guest_script(openwrt_host, "scripts/openwrt/install_xp2p_from_pages.sh")
    assert result.rc == 0, (
        "Install script integration test failed.\n"
        f"STDOUT:\n{result.stdout}\n"
        f"STDERR:\n{result.stderr}\n"
    )

