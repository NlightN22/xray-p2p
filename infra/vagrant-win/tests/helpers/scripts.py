from __future__ import annotations

import os
import shlex
from typing import Dict, Tuple

from .constants import (
    CLIENT_SCRIPT_URL,
    SERVER_CERT_APPLY_URL,
    SERVER_CERT_PATHS_URL,
    SERVER_CERT_SELFSIGNED_URL,
    SERVER_REVERSE_URL,
    SERVER_SCRIPT_URL,
    SERVER_USER_URL,
)
from .utils import run_checked

_SCRIPT_CACHE: Dict[str, str] = {}

_SCRIPT_SPECS: Dict[str, Tuple[str, str]] = {
    "server": ("/tmp/server.sh", SERVER_SCRIPT_URL),
    "client": ("/tmp/client.sh", CLIENT_SCRIPT_URL),
    "server_user": ("/tmp/server_user.sh", SERVER_USER_URL),
    "server_reverse": ("/tmp/server_reverse.sh", SERVER_REVERSE_URL),
    "common_loader": ("/tmp/common_loader.sh", "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/lib/common_loader.sh"),
    "common": ("/tmp/common.sh", "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/lib/common.sh"),
    "cert_apply": ("/tmp/server_install_cert_apply.sh", SERVER_CERT_APPLY_URL),
    "cert_selfsigned": ("/tmp/server_install_cert_selfsigned.sh", SERVER_CERT_SELFSIGNED_URL),
    "cert_paths": ("/tmp/server_cert_paths.sh", SERVER_CERT_PATHS_URL),
}


def _ensure_download(host, path: str, url: str):
    directory = os.path.dirname(path)
    if directory:
        run_checked(host, f"mkdir -p {shlex.quote(directory)}", f"ensure directory {directory}")
    result = host.run(f"test -x {shlex.quote(path)}")
    if result.rc == 0:
        return

    quoted_url = shlex.quote(url)
    quoted_path = shlex.quote(path)
    download_cmd = (
        f"set -e; "
        f"if command -v curl >/dev/null 2>&1; then "
        f"curl -fsSL {quoted_url} -o {quoted_path}; "
        f"elif command -v wget >/dev/null 2>&1; then "
        f"wget -q -O {quoted_path} {quoted_url}; "
        "else "
        "echo 'Neither curl nor wget available' >&2; "
        "exit 1; "
        "fi"
    )
    run_checked(host, download_cmd, f"download script {url}")
    run_checked(host, f"chmod +x {quoted_path}", f"mark script executable {url}")


def _ensure_script(host, key: str) -> str:
    cached = _SCRIPT_CACHE.get(key)
    if cached:
        if host.run(f"test -x {shlex.quote(cached)}").rc == 0:
            return cached
    path, url = _SCRIPT_SPECS[key]
    _ensure_download(host, path, url)
    _SCRIPT_CACHE[key] = path
    return path


def server_script_path(host) -> str:
    return _ensure_script(host, "server")


def client_script_path(host) -> str:
    return _ensure_script(host, "client")


def server_user_script_path(host) -> str:
    return _ensure_script(host, "server_user")


def server_reverse_script_path(host) -> str:
    return _ensure_script(host, "server_reverse")


def server_cert_apply_script_path(host) -> str:
    return _ensure_script(host, "cert_apply")


def server_cert_selfsigned_script_path(host) -> str:
    return _ensure_script(host, "cert_selfsigned")


def server_cert_paths_script_path(host) -> str:
    return _ensure_script(host, "cert_paths")


def common_loader_script_path(host) -> str:
    return _ensure_script(host, "common_loader")


def common_script_path(host) -> str:
    return _ensure_script(host, "common")


__all__ = [
    "server_script_path",
    "client_script_path",
    "server_user_script_path",
    "server_reverse_script_path",
    "server_cert_apply_script_path",
    "server_cert_selfsigned_script_path",
    "server_cert_paths_script_path",
]
