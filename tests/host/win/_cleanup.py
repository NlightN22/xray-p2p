from collections.abc import Iterable
from pathlib import Path

from testinfra.host import Host


def cleanup_xp2p_install(
    host: Host,
    *,
    config_dirs: Iterable[Path],
    state_files: Iterable[Path],
    extra_paths: Iterable[Path] = (),
) -> None:
    from . import env as _env

    targets: list[Path | str] = []
    extra_state = [
        _env.CONFIG_ROOT / ".apply",
        _env.CONFIG_ROOT / _env.APPLY_DIR_NAME,
        _env.CONFIG_ROOT / _env.APPLY_DIR_NAME / "apply.request",
        _env.CONFIG_ROOT / _env.APPLY_DIR_NAME / "apply.error",
        _env.CONFIG_ROOT / "xp2p-client.toml",
        _env.CONFIG_ROOT / "xp2p-server.toml",
        _env.CONFIG_ROOT / "xp2p-client.toml.lkg",
        _env.CONFIG_ROOT / "xp2p-server.toml.lkg",
        _env.CONFIG_ROOT / "xp2p-client.state.json",
        _env.CONFIG_ROOT / "xp2p-server.state.json",
        _env.CONFIG_ROOT / "xp2p-client.state.json.lkg",
        _env.CONFIG_ROOT / "xp2p-server.state.json.lkg",
        _env.CONFIG_ROOT / "xp2p-client.tun-full.json",
        _env.CONFIG_ROOT / "xp2p-server.tun-full.json",
        _env.CONFIG_ROOT / "state-heartbeat.json",
        _env.CONFIG_ROOT / "state-heartbeat-client.json",
        _env.CONFIG_ROOT / "state-heartbeat-server.json",
        _env.CONFIG_PENDING_ROOT,
        _env.CONFIG_LIVE_ROOT,
        _env.CONFIG_LKG_ROOT,
        _env.CLIENT_PENDING_DIR,
        _env.SERVER_PENDING_DIR,
        _env.CLIENT_LIVE_DIR,
        _env.SERVER_LIVE_DIR,
        _env.CLIENT_CONFIG_DIR,
        _env.SERVER_CONFIG_DIR,
        _env.CLIENT_CONFIG_DIR / "inbounds.json.lkg",
        _env.CLIENT_CONFIG_DIR / "outbounds.json.lkg",
        _env.CLIENT_CONFIG_DIR / "routing.json.lkg",
        _env.CLIENT_CONFIG_DIR / "logs.json.lkg",
        _env.SERVER_CONFIG_DIR / "inbounds.json.lkg",
        _env.SERVER_CONFIG_DIR / "outbounds.json.lkg",
        _env.SERVER_CONFIG_DIR / "routing.json.lkg",
        _env.SERVER_CONFIG_DIR / "logs.json.lkg",
        _env.LOGS_DIR,
    ]
    for path in [*config_dirs, *state_files, *extra_paths, *extra_state]:
        resolved = _env._as_path(path)
        pending = _env._pending_candidate(resolved)
        targets.append(pending)
        if pending != resolved:
            targets.append(resolved)
    _env.remove_paths(host, targets)


def cleanup_xp2p_leftovers(host: Host) -> None:
    from . import env as _env

    _env.remove_services(host, ["xp2p-client", "xp2p-server"])
    _env.remove_paths(
        host,
        [
            _env.PROGRAM_FILES_INSTALL_DIR,
            _env.PROGRAM_FILES_X86_INSTALL_DIR,
            _env.PROGRAM_DATA_ROOT,
        ],
    )

