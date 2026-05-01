from __future__ import annotations

import time

from testinfra.host import Host

from ._guest_scripts import run_guest_script, run_guest_script_with_env
from ._util import _install_marker


def ensure_xp2p_installed(machine: str, host: Host) -> dict[str, str]:
    from . import env as _env

    exists_check = host.run("test -f /srv/xray-p2p/scripts/build/build_deb_xp2p.sh")
    if exists_check.rc != 0:
        raise RuntimeError(
            "build_deb_xp2p.sh is missing on the guest. "
            "Fix the repository sync before running tests."
        )

    install_timeout = 600
    if _env._DEB_BUILD_READY:
        timing_label = f"linux install_xp2p {machine} (skip_build)"
        start = time.perf_counter()
        result = run_guest_script_with_env(
            host,
            "scripts/linux/install_xp2p.sh",
            {"XP2P_SKIP_BUILD": "1"},
            timeout=install_timeout,
        )
        print(f"TIMING: {timing_label}: {time.perf_counter() - start:.2f}s")
    else:
        timing_label = f"linux install_xp2p {machine} (build)"
        start = time.perf_counter()
        result = run_guest_script(
            host,
            "scripts/linux/install_xp2p.sh",
            timeout=install_timeout,
        )
        print(f"TIMING: {timing_label}: {time.perf_counter() - start:.2f}s")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to build and install xp2p on guest "
            f"{machine} (exit {result.rc}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

    source_version = _install_marker("__XP2P_SOURCE_VERSION__=", result.stdout)
    installed_version = _install_marker("__XP2P_INSTALLED_VERSION__=", result.stdout)
    if not source_version or not installed_version:
        raise RuntimeError(
            "xp2p install script did not emit expected markers.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

    versions = {"source": source_version, "installed": installed_version}
    _env._VERSION_CACHE[machine] = versions
    _env._DEB_BUILD_READY = True
    return versions


def ensure_xp2p_installed_cached(machine: str, host: Host) -> dict[str, str]:
    from . import env as _env

    cached = _env._VERSION_CACHE.get(machine)
    if cached is not None:
        return cached
    return ensure_xp2p_installed(machine, host)
