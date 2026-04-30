import os
from pathlib import Path

from tests.host import common

REPO_ROOT = common.REPO_ROOT
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "windows10"
DEFAULT_SERVER = "win10-a"
DEFAULT_CLIENT = "win10-b"
DEFAULT_TARGET = "10.62.10.21"

PROGRAM_FILES_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
PROGRAM_FILES_X86_INSTALL_DIR = Path(r"C:\Program Files (x86)\xp2p")
PROGRAM_DATA_ROOT = Path(os.environ.get("ProgramData", r"C:\ProgramData")) / "xp2p"
CONFIG_ROOT = Path(os.environ.get("XP2P_CONFIG_ROOT", str(PROGRAM_DATA_ROOT)))

APPLY_DIR_NAME = ".state"
PENDING_DIR_NAME = "pending"
LIVE_DIR_NAME = "live"
LKG_DIR_NAME = "lkg"
CLIENT_CONFIG_DIR_NAME = "config-client"
SERVER_CONFIG_DIR_NAME = "config-server"

CLIENT_CONFIG_DIR = CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_CONFIG_DIR = CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
CONFIG_PENDING_ROOT = CONFIG_ROOT / APPLY_DIR_NAME / PENDING_DIR_NAME
CONFIG_LIVE_ROOT = CONFIG_ROOT / APPLY_DIR_NAME / LIVE_DIR_NAME
CONFIG_LKG_ROOT = CONFIG_ROOT / APPLY_DIR_NAME / LKG_DIR_NAME
CLIENT_PENDING_DIR = CONFIG_PENDING_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_PENDING_DIR = CONFIG_PENDING_ROOT / SERVER_CONFIG_DIR_NAME
CLIENT_LIVE_DIR = CONFIG_LIVE_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_LIVE_DIR = CONFIG_LIVE_ROOT / SERVER_CONFIG_DIR_NAME

LOGS_DIR = Path(os.environ.get("XP2P_LOG_ROOT", str(CONFIG_ROOT / "logs")))
LOG_ROOT = LOGS_DIR

XP2P_EXE = PROGRAM_FILES_INSTALL_DIR / "xp2p.exe"
SERVICE_START_TIMEOUT = 60

GUEST_TESTS_ROOT = Path(r"C:\xp2p\tests\guest")
LOCAL_GUEST_TESTS_ROOT = REPO_ROOT / "tests" / "guest"
GUEST_BUILD_ROOT = Path(r"C:\xp2p\build")

MSI_MARKER = "__MSI_PATH__="
MSI_CACHE_DIR_X64 = Path(r"C:\xp2p\build\msi-cache")
MSI_CACHE_DIR_X86 = Path(r"C:\xp2p\build\msi-cache-x86")
PROJECT_SYNC_MARKER = Path(r"C:\xp2p\scripts\build\build_and_install_msi.ps1")
XP2P_UNINSTALL_SCRIPT = Path(r"C:\xp2p\scripts\windows\uninstall_xp2p.ps1")

DEFAULT_POWERSHELL_TIMEOUT = 120
DEFAULT_GUEST_SCRIPT_TIMEOUT = 120
DEFAULT_XP2P_COMMAND_TIMEOUT = 120
ADMIN_XP2P_SUBCOMMANDS = {"run", "service"}

WINTUN_DLL_SOURCE_BUNDLE_X64 = Path(r"C:\xp2p\distro\windows\bundle\x86_64\wintun.dll")
WINTUN_DLL_SOURCE_MSI_BIN_X64 = Path(r"C:\xp2p\build\msi-bin\bundle\wintun.dll")

_MSI_CACHE_PATH_X64: str | None = None
_MSI_CACHE_PATH_X86: str | None = None
_MSI_BUILD_ID: str | None = None
_GUEST_SCRIPT_CACHE: dict[tuple[str, str], str] = {}

WIN_STACKS = {
    "win7": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "windows7",
        "server": "win7-a",
        "client": "win7-b",
        "target": "10.62.10.61",
    },
    "win10": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "windows10",
        "server": "win10-a",
        "client": "win10-b",
        "target": "10.62.10.21",
    },
    "win2016": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "server2016",
        "server": "win2016-a",
        "client": "win2016-b",
        "target": "10.62.10.51",
    },
    "win2022": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "server2022",
        "server": "win2022-a",
        "client": "win2022-b",
        "target": "10.62.10.31",
    },
}

_CURRENT_WIN_STACK = "win10"

from ._cleanup import cleanup_xp2p_install, cleanup_xp2p_leftovers
from ._dump import _sanitize_dump_label, dump_failure_state
from ._fs import (
    _as_path,
    _path_exists_guest,
    _path_exists_raw,
    _pending_candidate,
    _resolve_config_path,
    get_remote_file_size,
    path_exists,
    paths_exist,
    pending_candidate,
    read_text,
    read_toml,
    remove_path,
    remove_paths,
    resolve_config_path,
    write_apply_request,
    write_text,
    write_text_exact,
)
from ._net_ipv4 import ensure_default_ipv4_route, get_default_ipv4_sendthrough, get_host_ipv4
from ._net_routes import get_interface_index, get_net_routes, remove_tun_adapters
from ._package import (
    MsiServiceUnavailable,
    _build_msi_package,
    _cleanup_orphaned_xp2p_msi,
    _msi_build_markers,
    _manual_install_from_msi_bin,
    _read_msi_failure_context,
    _read_msi_log_tail,
    ensure_msi_package,
    ensure_msi_package_x86,
    ensure_program_files_install,
    ensure_wintun_dll,
    install_xp2p_from_msi,
    purge_xp2p_install,
    set_msi_build_id,
    uninstall_xp2p_from_msi,
)
from ._services import remove_services, service_exists, stop_xp2p_processes
from ._sh import (
    _guest_script_cache_key,
    _missing_script_error,
    _ps_quote,
    _refresh_ssh_host,
    _remote_sha256,
    _sha256_bytes,
    _ssh_run_with_refresh,
    _stage_guest_script,
    encode_powershell,
    ps_quote,
    run_guest_script,
    run_powershell,
)
from ._sync import _wait_for_sync_marker, ensure_project_synced
from ._vagrant import (
    available_win_stacks,
    ensure_machine_running,
    get_ssh_host,
    require_vagrant_environment,
    set_win_stack,
)
from ._xp2p import (
    _admin_token_marker,
    _detect_xp2p_exe,
    _encode_args_payload,
    _extract_marker,
    _query_install_location,
    _run_xp2p_admin,
    _run_xp2p_direct,
    _run_xp2p_with_timeout_marker,
    _search_user_programs,
    _set_install_paths_from_exe,
    _xp2p_requires_admin,
    _xp2p_timeout_marker,
    ensure_admin_token,
    find_xp2p_exe,
    get_program_files_install_dir,
    run_xp2p,
)

