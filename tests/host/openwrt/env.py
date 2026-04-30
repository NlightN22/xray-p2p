from __future__ import annotations

from tests.host import common
from tests.host.openwrt._fs import _posix, _read_file_safe
from tests.host.openwrt._package import (
    IPK_OUTPUT_DIR,
    IPK_OUTPUT_POSIX,
    _assert_default_configs_present,
    _purge_xp2p_artifacts,
    bootstrap_xp2p_configs,
    build_ipk,
    cleanup_xp2p,
    ensure_packages_index_present,
    install_ipk_on_host,
    latest_local_ipk,
    opkg_install_local,
    opkg_remove,
    stage_ipk_on_guest,
)
from tests.host.openwrt._sh import (
    ALPINE_DNSMASQ_INSTALLER,
    ALPINE_GUEST_SCRIPTS_ROOT,
    GUEST_SCRIPTS_HASH_FILE,
    GUEST_SCRIPTS_ROOT,
    GUEST_SCRIPTS_SOURCE,
    _compute_guest_scripts_hash,
    _provision_file,
    _provision_guest_scripts,
    _read_cached_scripts_hash,
    _write_cached_scripts_hash,
    ensure_guest_scripts_synced,
    run_alpine_guest_script,
    run_guest_script,
    stop_process,
)
from tests.host.openwrt._vagrant import (
    ALPINE_MACHINES,
    BUILDER_MACHINE,
    BUILDER_VAGRANT_DIR,
    DEFAULT_OPENWRT_MACHINE,
    OPENWRT_MACHINES,
    OPENWRT_VAGRANT_DIR,
    REPO_ROOT,
    WORKTREE_POSIX,
    alpine_host_factory,
    get_alpine_host,
    get_ipk_builder_host,
    get_openwrt_host,
    host_factory,
    require_ipk_builder_environment,
    require_openwrt_environment,
    sync_build_output,
)
from tests.host.openwrt._xp2p import (
    DEFAULT_TARGET,
    TARGET_ENV_VAR,
    _kill_port_listeners,
    _netstat_snapshot,
    _stop_xp2p_services,
    resolve_target_from_env,
    run_xp2p,
    run_xp2p_live,
    run_xp2p_with_env,
    xp2p_run_session,
)

__all__ = [
    "ALPINE_DNSMASQ_INSTALLER",
    "ALPINE_GUEST_SCRIPTS_ROOT",
    "ALPINE_MACHINES",
    "BUILDER_MACHINE",
    "BUILDER_VAGRANT_DIR",
    "DEFAULT_OPENWRT_MACHINE",
    "DEFAULT_TARGET",
    "GUEST_SCRIPTS_HASH_FILE",
    "GUEST_SCRIPTS_ROOT",
    "GUEST_SCRIPTS_SOURCE",
    "IPK_OUTPUT_DIR",
    "IPK_OUTPUT_POSIX",
    "OPENWRT_MACHINES",
    "OPENWRT_VAGRANT_DIR",
    "REPO_ROOT",
    "TARGET_ENV_VAR",
    "WORKTREE_POSIX",
    "_SCRIPTS_HASH_CACHE",
    "_SCRIPTS_SYNCED",
    "_assert_default_configs_present",
    "_compute_guest_scripts_hash",
    "_kill_port_listeners",
    "_netstat_snapshot",
    "_posix",
    "_provision_file",
    "_provision_guest_scripts",
    "_purge_xp2p_artifacts",
    "_read_cached_scripts_hash",
    "_read_file_safe",
    "_stop_xp2p_services",
    "_write_cached_scripts_hash",
    "alpine_host_factory",
    "bootstrap_xp2p_configs",
    "build_ipk",
    "cleanup_xp2p",
    "ensure_guest_scripts_synced",
    "ensure_packages_index_present",
    "get_alpine_host",
    "get_ipk_builder_host",
    "get_openwrt_host",
    "host_factory",
    "install_ipk_on_host",
    "latest_local_ipk",
    "opkg_install_local",
    "opkg_remove",
    "require_ipk_builder_environment",
    "require_openwrt_environment",
    "resolve_target_from_env",
    "run_alpine_guest_script",
    "run_guest_script",
    "run_xp2p",
    "run_xp2p_live",
    "run_xp2p_with_env",
    "stage_ipk_on_guest",
    "stop_process",
    "sync_build_output",
    "xp2p_run_session",
]


def __getattr__(name: str):
    if name in {"_SCRIPTS_HASH_CACHE", "_SCRIPTS_SYNCED"}:
        from tests.host.openwrt import _sh

        return getattr(_sh, name)
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")


def __dir__() -> list[str]:
    return sorted(set(globals().keys()) | {"_SCRIPTS_HASH_CACHE", "_SCRIPTS_SYNCED"})
