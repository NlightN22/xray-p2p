import time
from collections.abc import Callable

from testinfra.host import Host


def ensure_program_files_install(
    host: Host,
    *,
    force_reinstall: bool = False,
    machine: str | None = None,
    reconnect: Callable[[], Host] | None = None,
) -> Host:
    from . import env as _env

    def _ensure_xray_present() -> None:
        expected = _env.PROGRAM_FILES_INSTALL_DIR / "bin" / "xray.exe"
        if _env.path_exists(host, expected):
            return
        _env._manual_install_from_msi_bin(host)
        if not _env.path_exists(host, expected):
            raise RuntimeError(f"xray.exe missing after install: {expected}")

    if machine is not None:
        if reconnect is None:
            reconnect = lambda: _env.get_ssh_host(machine)
        host = _env.ensure_project_synced(host, machine=machine, reconnect=reconnect)
    if not force_reinstall:
        detected = _env._detect_xp2p_exe(host)
        if detected is not None:
            _env._set_install_paths_from_exe(detected)
            _ensure_xray_present()
            return host

    start = time.perf_counter()
    msi_path = _env.ensure_msi_package(host, machine=machine, reconnect=reconnect)
    print(f"TIMING: ensure_msi_package: {time.perf_counter() - start:.2f}s")
    start = time.perf_counter()
    try:
        _env.install_xp2p_from_msi(host, msi_path)
    except _env.MsiServiceUnavailable:
        detected = _env._detect_xp2p_exe(host)
        if detected is not None:
            _env._set_install_paths_from_exe(detected)
            _ensure_xray_present()
            return host
        _env._manual_install_from_msi_bin(host)
        detected = _env._detect_xp2p_exe(host)
        if detected is not None:
            _env._set_install_paths_from_exe(detected)
            _ensure_xray_present()
            return host
        raise
    except RuntimeError as exc:
        detected = _env._detect_xp2p_exe(host)
        if detected is not None:
            print(f"WARNING: MSI install reported failure, but xp2p.exe exists at {detected}. {exc}")
            _env._set_install_paths_from_exe(detected)
            _ensure_xray_present()
            return host
        try:
            _env._manual_install_from_msi_bin(host)
        except Exception:
            raise
        detected = _env._detect_xp2p_exe(host)
        if detected is not None:
            print(f"WARNING: Using xp2p.exe from MSI bin install at {detected}. {exc}")
            _env._set_install_paths_from_exe(detected)
            _ensure_xray_present()
            return host
        raise
    print(f"TIMING: install_xp2p_from_msi: {time.perf_counter() - start:.2f}s")

    start = time.perf_counter()
    detected = _env._detect_xp2p_exe(host)
    print(f"TIMING: detect_xp2p_exe: {time.perf_counter() - start:.2f}s")
    if detected is None:
        raise RuntimeError(
            "xp2p.exe not found after MSI installation on remote host. "
            f"Checked: {_env.PROGRAM_FILES_INSTALL_DIR} and {_env.PROGRAM_FILES_X86_INSTALL_DIR}."
        )
    _env._set_install_paths_from_exe(detected)
    _ensure_xray_present()
    return host
