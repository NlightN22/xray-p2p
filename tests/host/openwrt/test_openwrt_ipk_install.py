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


@pytest.fixture(scope="session")
def openwrt_ipk_target() -> str:
    return openwrt_env.resolve_target_from_env()


@pytest.fixture(scope="session")
def xp2p_openwrt_ipk(ipk_builder_host, openwrt_ipk_target):
    openwrt_env.IPK_OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    openwrt_env.build_ipk(ipk_builder_host, openwrt_ipk_target)
    artifact = openwrt_env.latest_local_ipk()
    assert artifact, "Expected build/ipk to contain a freshly built xp2p ipk"
    openwrt_env.ensure_packages_index_present()
    openwrt_env.sync_build_output()
    return artifact


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_ipk_can_be_installed(openwrt_host, xp2p_openwrt_ipk):
    staged_path = openwrt_env.stage_ipk_on_guest(openwrt_host, xp2p_openwrt_ipk, REMOTE_IPK_PATH)
    openwrt_env.opkg_remove(openwrt_host, "xp2p", ignore_missing=True)
    openwrt_env.opkg_install_local(openwrt_host, staged_path)

    version = openwrt_env.run_xp2p(openwrt_host, "--version")
    assert version.rc == 0, f"xp2p --version failed:\nSTDOUT:\n{version.stdout}\nSTDERR:\n{version.stderr}"
    assert (version.stdout or "").strip(), "xp2p --version did not emit any output"
