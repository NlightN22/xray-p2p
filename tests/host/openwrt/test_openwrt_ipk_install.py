from __future__ import annotations

"""
OpenWrt packaging smoke-test.

The test invokes scripts/build/build_openwrt_ipk.sh inside the Debian builder VM
(`infra/vagrant/debian12/ipk-build`) and installs the resulting ipk on
`infra/vagrant/openwrt`. Override the build target via the
`XP2P_OPENWRT_IPK_TARGET` environment variable when you need an arch other than
linux-amd64. Artifacts land in `build/ipk`, which is the same directory copied
into `/tmp/build-openwrt` by `infra/vagrant/openwrt/Vagrantfile` during
provisioning.
"""

from pathlib import PurePosixPath

import pytest

from tests.host.openwrt import env as openwrt_env

REMOTE_IPK_PATH = PurePosixPath("/tmp/xp2p.ipk")


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_ipk_can_be_installed(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.sync_build_output(openwrt_env.DEFAULT_OPENWRT_MACHINE)
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, destination=REMOTE_IPK_PATH, force=True)
    version = openwrt_env.run_xp2p(openwrt_host, "--version")
    assert version.rc == 0, f"xp2p --version failed:\nSTDOUT:\n{version.stdout}\nSTDERR:\n{version.stderr}"
    assert (version.stdout or "").strip(), "xp2p --version did not emit any output"
