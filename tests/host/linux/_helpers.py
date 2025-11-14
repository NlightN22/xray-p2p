from __future__ import annotations

from pathlib import PurePosixPath

from testinfra.host import Host

from tests.host.linux import env as linux_env

INSTALL_ROOT = PurePosixPath("/etc/xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
SERVER_CONFIG_DIR_NAME = "config-server"
CLIENT_CONFIG_DIR = INSTALL_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_CONFIG_DIR = INSTALL_ROOT / SERVER_CONFIG_DIR_NAME
CLIENT_STATE_FILES = [
    INSTALL_ROOT / "install-state-client.json",
    INSTALL_ROOT / "install-state.json",
]
SERVER_STATE_FILES = [
    INSTALL_ROOT / "install-state-server.json",
    INSTALL_ROOT / "install-state.json",
]
LOG_ROOT = PurePosixPath("/var/log/xp2p")
CLIENT_LOG_FILE = LOG_ROOT / "client.err"
SERVER_LOG_FILE = LOG_ROOT / "server.err"
XRAY_BINARY = INSTALL_ROOT / "bin" / "xray"


def cleanup_client_install(host: Host, runner) -> None:
    runner(
        "client",
        "remove",
        "--path",
        INSTALL_ROOT.as_posix(),
        "--config-dir",
        CLIENT_CONFIG_DIR_NAME,
        "--ignore-missing",
    )


def cleanup_server_install(host: Host, runner) -> None:
    runner(
        "server",
        "remove",
        "--path",
        INSTALL_ROOT.as_posix(),
        "--config-dir",
        SERVER_CONFIG_DIR_NAME,
        "--ignore-missing",
    )


def read_json(host: Host, path: PurePosixPath) -> dict:
    return linux_env.read_json(host, path)


def read_text(host: Host, path: PurePosixPath) -> str:
    return linux_env.read_text(host, path)


def path_exists(host: Host, path: PurePosixPath) -> bool:
    return linux_env.path_exists(host, path)


def remove_path(host: Host, path: PurePosixPath) -> None:
    linux_env.remove_path(host, path)


def write_text(host: Host, path: PurePosixPath, content: str) -> None:
    linux_env.write_text(host, path, content)


def file_sha256(host: Host, path: PurePosixPath) -> str:
    return linux_env.file_sha256(host, path)
