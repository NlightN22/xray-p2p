from __future__ import annotations

from pathlib import PurePosixPath

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env


CERT_DEST = PurePosixPath("/etc/xp2p/tls/server/cert.pem")
KEY_DEST = PurePosixPath("/etc/xp2p/tls/server/key.pem")


def _xp2p(host, *args: str, check: bool = False):
    result = openwrt_env.run_xp2p_live(host, *args)
    if check and result.rc != 0:
        pytest.fail(
            f"xp2p command failed (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _copy_remote_file(host, source: PurePosixPath, dest: PurePosixPath) -> None:
    result = openwrt_env.run_guest_script(
        host,
        "scripts/linux/copy_file.sh",
        source.as_posix(),
        dest.as_posix(),
    )
    if result.rc != 0:
        pytest.fail(
            f"Failed to copy {source} to {dest} (exit {result.rc}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _remove_path(host, path: PurePosixPath) -> None:
    result = openwrt_env.run_guest_script(host, "scripts/linux/remove_path.sh", path.as_posix())
    if result.rc not in (0, 3):
        pytest.fail(
            f"Failed to remove {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_cert_set_quiet_replaces_existing_certificate(openwrt_server_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    run = lambda *cmd, check=False: _xp2p(openwrt_server_host, *cmd, check=check)
    cert_source = PurePosixPath("/tmp/xp2p-quiet-cert.pem")
    key_source = PurePosixPath("/tmp/xp2p-quiet-key.pem")
    install_dir = helpers.INSTALL_ROOT.as_posix()
    config_dir = helpers.SERVER_CONFIG_DIR_NAME

    try:
        helpers.cleanup_server_install(openwrt_server_host, run)
        run(
            "server",
            "install",
            "--path",
            install_dir,
            "--config-dir",
            config_dir,
            "--host",
            "quiet-source.openwrt.test",
            "--force",
            check=True,
        )
        helpers.ensure_service_running(openwrt_server_host, "server")
        helpers.wait_for_live_config(openwrt_server_host, "server")
        _copy_remote_file(openwrt_server_host, CERT_DEST, cert_source)
        _copy_remote_file(openwrt_server_host, KEY_DEST, key_source)
        expected_hash = helpers.file_sha256(openwrt_server_host, cert_source)

        helpers.cleanup_server_install(openwrt_server_host, run)
        run(
            "server",
            "install",
            "--path",
            install_dir,
            "--config-dir",
            config_dir,
            "--host",
            "quiet-target.openwrt.test",
            "--force",
            check=True,
        )

        result = run(
            "server",
            "cert",
            "set",
            "--path",
            install_dir,
            "--config-dir",
            config_dir,
            "--cert",
            cert_source.as_posix(),
            "--key",
            key_source.as_posix(),
            "--quiet",
        )
        assert result.rc == 0, (
            f"Expected quiet cert set to succeed, rc={result.rc}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
        combined = f"{result.stdout}\n{result.stderr}".lower()
        assert "replace existing certificate" not in combined
        assert helpers.file_sha256(openwrt_server_host, CERT_DEST) == expected_hash
    finally:
        helpers.cleanup_server_install(openwrt_server_host, run)
        _remove_path(openwrt_server_host, cert_source)
        _remove_path(openwrt_server_host, key_source)
