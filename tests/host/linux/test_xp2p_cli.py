from __future__ import annotations

import pytest

from tests.host.linux import env as linux_env


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize("machine", linux_env.MACHINE_IDS, ids=linux_env.MACHINE_IDS)
def test_xp2p_cli_is_installed_and_reports_version(machine: str, linux_host_factory):
    host = linux_host_factory(machine)
    versions = linux_env.ensure_xp2p_installed(machine, host)

    source_version = versions["source"].strip()
    installed_version = versions["installed"].strip()
    assert source_version, "xp2p --version from go run returned empty output"
    assert installed_version, "Installed xp2p binary reported empty version"
    assert installed_version == source_version, (
        f"Installed xp2p version {installed_version!r} "
        f"does not match source version {source_version!r}"
    )

    version_check = linux_env.run_xp2p(host, "--version")
    assert version_check.rc == 0, (
        f"xp2p --version failed on {machine}.\n"
        f"STDOUT:\n{version_check.stdout}\nSTDERR:\n{version_check.stderr}"
    )
    assert (version_check.stdout or "").strip() == source_version, (
        "xp2p --version output mismatch for installed binary"
    )
