from __future__ import annotations

import os
from pathlib import Path, PurePosixPath
import shutil
import urllib.request


REPO_ROOT = Path(__file__).resolve().parents[3]
DEB_ROOT = REPO_ROOT / "build" / "deb"
PREVIOUS_DEB_ENV = "XP2P_LINUX_PREVIOUS_DEB"
RELEASE_BASE_URL = "https://github.com/NlightN22/xray-p2p/releases/download"


def ensure_previous_release_deb(version: str) -> Path:
    asset = f"xp2p_{version}_amd64.deb"
    cached = DEB_ROOT / "previous" / asset
    override = os.environ.get(PREVIOUS_DEB_ENV, "").strip()
    if override:
        source = Path(override).expanduser().resolve()
        if not source.is_file():
            raise AssertionError(f"{PREVIOUS_DEB_ENV} does not point to a file: {source}")
        cached.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, cached)
        return cached
    if cached.is_file() and cached.stat().st_size > 0:
        return cached

    cached.parent.mkdir(parents=True, exist_ok=True)
    url = f"{RELEASE_BASE_URL}/v{version}/{asset}"
    partial = cached.with_suffix(".partial")
    try:
        urllib.request.urlretrieve(url, partial)
        partial.replace(cached)
    except Exception as exc:
        partial.unlink(missing_ok=True)
        raise RuntimeError(
            f"Failed to obtain previous Linux package from {url}. "
            f"Set {PREVIOUS_DEB_ENV} to a local release DEB."
        ) from exc
    return cached


def current_candidate_deb() -> Path:
    candidates = sorted(
        (DEB_ROOT / "artifacts").glob("xp2p_*_amd64.deb"),
        key=lambda path: path.stat().st_mtime,
    )
    if not candidates:
        raise AssertionError("Current Linux DEB candidate is missing")
    return candidates[-1]


def guest_path(path: Path) -> PurePosixPath:
    relative = path.resolve().relative_to(REPO_ROOT.resolve())
    return PurePosixPath("/srv/xray-p2p") / PurePosixPath(relative.as_posix())


def install_deb(host, path: Path) -> None:
    result = host.run(f"sudo -n dpkg -i {guest_path(path)}")
    assert result.rc == 0, f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"


def add_hosts_entry(host, address: str, name: str, marker: str) -> None:
    remove_hosts_entry(host, marker)
    host.run(
        f"printf '%s %s {marker}\\n' '{address}' '{name}' | "
        "sudo -n tee -a /etc/hosts >/dev/null"
    )


def remove_hosts_entry(host, marker: str) -> None:
    host.run(f"sudo -n sed -i '\\|{marker}$|d' /etc/hosts")


def detect_host_ipv4(host) -> str:
    result = host.run("ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1")
    assert result.rc == 0, result.stderr
    addresses = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    assert addresses, "No global IPv4 address found"
    return next(
        (address for address in addresses if not address.startswith("10.0.2.")),
        addresses[0],
    )
