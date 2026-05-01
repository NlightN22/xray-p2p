from __future__ import annotations

from ._guest_scripts import run_guest_script, run_guest_script_with_env
from ._package_install import ensure_xp2p_installed, ensure_xp2p_installed_cached
from ._process import kill_xp2p_processes, stop_process
from ._remote_fs import file_sha256, path_exists, read_json, read_text, remove_path, write_text
from ._sessions import xp2p_run_session, xp2p_run_session_with_env
from ._sh import _run_shell
from ._util import _install_marker, _posix
from ._vagrant import (
    DEFAULT_AUX,
    DEFAULT_CLIENT,
    DEFAULT_SERVER,
    MACHINE_IDS,
    REPO_ROOT,
    VAGRANT_DIR,
    ensure_machine_running,
    get_ssh_host,
    machine_host_factory,
    require_vagrant_environment,
)
from ._xp2p_cli import run_xp2p, run_xp2p_with_env
from ._xp2p_paths import GUEST_SCRIPTS_ROOT, INSTALL_PATH, WORK_TREE

_VERSION_CACHE: dict[str, dict[str, str]] = {}
_DEB_BUILD_READY = False

__all__ = [
    "DEFAULT_AUX",
    "DEFAULT_CLIENT",
    "DEFAULT_SERVER",
    "GUEST_SCRIPTS_ROOT",
    "INSTALL_PATH",
    "MACHINE_IDS",
    "REPO_ROOT",
    "VAGRANT_DIR",
    "WORK_TREE",
    "_DEB_BUILD_READY",
    "_VERSION_CACHE",
    "_install_marker",
    "_posix",
    "_run_shell",
    "ensure_machine_running",
    "ensure_xp2p_installed",
    "ensure_xp2p_installed_cached",
    "file_sha256",
    "get_ssh_host",
    "kill_xp2p_processes",
    "machine_host_factory",
    "path_exists",
    "read_json",
    "read_text",
    "remove_path",
    "require_vagrant_environment",
    "run_guest_script",
    "run_guest_script_with_env",
    "run_xp2p",
    "run_xp2p_with_env",
    "stop_process",
    "write_text",
    "xp2p_run_session",
    "xp2p_run_session_with_env",
]
