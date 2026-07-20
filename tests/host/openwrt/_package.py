from __future__ import annotations

import os
import shlex
import shutil
import urllib.request
from pathlib import Path, PurePosixPath

from testinfra.host import Host

from tests.host.openwrt._vagrant import REPO_ROOT, WORKTREE_POSIX
from tests.host.openwrt._xp2p import _stop_xp2p_services

IPK_OUTPUT_DIR = REPO_ROOT / "build" / "ipk"
IPK_OUTPUT_POSIX = WORKTREE_POSIX / "build" / "ipk"
PREVIOUS_IPK_ENV = "XP2P_OPENWRT_PREVIOUS_IPK"
RELEASE_BASE_URL = "https://github.com/NlightN22/xray-p2p/releases/download"


def build_ipk(host: Host, target: str) -> None:
    worktree = WORKTREE_POSIX.as_posix()
    output_dir = IPK_OUTPUT_POSIX.as_posix()
    command = (
        f"cd {shlex.quote(worktree)} && "
        f"bash ./scripts/build/build_openwrt_ipk.sh "
        f"--target {shlex.quote(target)} "
        f"--output-dir {shlex.quote(output_dir)} "
        f"--force-build"
    )
    result = host.run(f"bash -lc {shlex.quote(command)}", timeout=600)
    if result.rc != 0:
        raise RuntimeError(
            "OpenWrt build script failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def latest_local_ipk() -> Path | None:
    if not IPK_OUTPUT_DIR.exists():
        return None
    candidates = list(IPK_OUTPUT_DIR.glob("xp2p_*.ipk"))
    if not candidates:
        return None
    candidates.sort(key=lambda path: path.stat().st_mtime)
    return candidates[-1]


def ensure_previous_release_ipk(version: str, target: str) -> Path:
    asset_name = f"xp2p_{version}-1_{target}.ipk"
    cached = IPK_OUTPUT_DIR / "previous" / asset_name
    override = os.environ.get(PREVIOUS_IPK_ENV, "").strip()
    if override:
        source = Path(override).expanduser().resolve()
        if not source.is_file():
            raise AssertionError(f"{PREVIOUS_IPK_ENV} does not point to a file: {source}")
        cached.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, cached)
        return cached
    if cached.is_file() and cached.stat().st_size > 0:
        return cached

    cached.parent.mkdir(parents=True, exist_ok=True)
    url = f"{RELEASE_BASE_URL}/v{version}/{asset_name}"
    partial = cached.with_suffix(".partial")
    try:
        urllib.request.urlretrieve(url, partial)
        partial.replace(cached)
    except Exception as exc:
        partial.unlink(missing_ok=True)
        raise RuntimeError(
            f"Failed to obtain previous OpenWrt package from {url}. "
            f"Set {PREVIOUS_IPK_ENV} to a local release IPK."
        ) from exc
    return cached


def ensure_packages_index_present() -> None:
    packages = IPK_OUTPUT_DIR / "Packages"
    packages_gz = IPK_OUTPUT_DIR / "Packages.gz"
    if not packages.exists():
        raise AssertionError(f"Expected Packages file at {packages}")
    if not packages_gz.exists():
        raise AssertionError(f"Expected Packages.gz file at {packages_gz}")


def stage_ipk_on_guest(host: Host, ipk_path: Path, destination: PurePosixPath | None = None) -> PurePosixPath:
    target_path = destination or PurePosixPath("/tmp/xp2p.ipk")
    try:
        relative_source = ipk_path.resolve().relative_to(IPK_OUTPUT_DIR.resolve())
    except ValueError:
        relative_source = Path(ipk_path.name)
    remote_source = PurePosixPath("/tmp/build-openwrt") / PurePosixPath(relative_source.as_posix())
    copy_command = f"cp {shlex.quote(remote_source.as_posix())} {shlex.quote(target_path.as_posix())}"
    result = host.run(copy_command)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to copy ipk from /tmp/build-openwrt.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return target_path


def install_ipk_on_host(
    host: Host,
    ipk_path: Path,
    *,
    destination: PurePosixPath | None = None,
    force: bool = True,
) -> PurePosixPath:
    dest = destination or PurePosixPath("/tmp/xp2p.ipk")
    skip_reinstall = os.environ.get("XP2P_OPENWRT_SKIP_REINSTALL", "").strip().lower() in {"1", "true", "yes"}
    if skip_reinstall:
        status = host.run("opkg status xp2p")
        if status.rc == 0:
            return dest
    staged_path = stage_ipk_on_guest(host, ipk_path, dest)
    opkg_remove(host, "xp2p", ignore_missing=True)
    _purge_xp2p_artifacts(host)
    opkg_install_local(host, staged_path)
    return staged_path


def bootstrap_xp2p_configs(host: Host) -> None:
    for role, config_dir in (("client", "config-client"), ("server", "config-server")):
        result = host.run(f"/usr/bin/xp2p {role} install --path /etc/xp2p --config-dir {config_dir}")
        if result.rc != 0:
            raise RuntimeError(
                f"xp2p {role} install failed on {host.backend.hostname} "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
    _assert_default_configs_present(host)


def cleanup_xp2p(host: Host) -> None:
    _stop_xp2p_services(host)
    host.run("OPKG_FORCE_REMOVE=1 opkg remove --autoremove xp2p >/dev/null 2>&1 || true")
    host.run("rm -f /tmp/xp2p-client.log /tmp/xp2p-server.log /tmp/xp2p.ipk >/dev/null 2>&1 || true")
    host.run("rm -f /etc/xp2p/dns-forward-state.json >/dev/null 2>&1 || true")
    _purge_xp2p_artifacts(host)


def _purge_xp2p_artifacts(host: Host) -> None:
    host.run("rm -rf /etc/xp2p /usr/bin/xp2p /usr/lib/xp2p >/dev/null 2>&1 || true")


def opkg_remove(host: Host, package: str, ignore_missing: bool = True) -> None:
    status = host.run(f"opkg status {shlex.quote(package)}")
    if status.rc != 0:
        if ignore_missing:
            return
        raise RuntimeError(
            f"Package {package} is not installed.\nSTDOUT:\n{status.stdout}\nSTDERR:\n{status.stderr}"
        )
    result = host.run(f"opkg remove {shlex.quote(package)}")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to remove package {package} "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def opkg_install_local(host: Host, path: PurePosixPath) -> None:
    result = host.run(f"opkg install --force-reinstall {shlex.quote(path.as_posix())}")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to install ipk {path} "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _assert_default_configs_present(host: Host) -> None:
    required = [
        "/etc/xp2p/config-client/inbounds.json",
        "/etc/xp2p/config-client/outbounds.json",
        "/etc/xp2p/config-client/routing.json",
        "/etc/xp2p/config-client/logs.json",
        "/etc/xp2p/config-server/inbounds.json",
        "/etc/xp2p/config-server/outbounds.json",
        "/etc/xp2p/config-server/routing.json",
        "/etc/xp2p/config-server/logs.json",
    ]
    missing: list[str] = []
    for path in required:
        if host.run(f"test -f {shlex.quote(path)}").rc != 0:
            missing.append(path)
    if missing:
        raise RuntimeError(f"xp2p default configs are missing on {host.backend.hostname}: {', '.join(missing)}")
