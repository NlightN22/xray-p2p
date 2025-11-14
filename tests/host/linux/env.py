from __future__ import annotations

import shlex
from pathlib import Path
from typing import Callable

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from tests.host import common

REPO_ROOT = common.REPO_ROOT
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "debian12" / "deb-build"
MACHINE_IDS: tuple[str, ...] = (
    "deb12-deb-build-a",
    "deb12-deb-build-b",
    "deb12-deb-build-c",
)
SYNCED_REPO = Path("/srv/xray-p2p")
WORK_TREE = Path("/home/vagrant/xray-p2p")
INSTALL_PATH = Path("/usr/local/bin/xp2p")

_VERSION_CACHE: dict[str, dict[str, str]] = {}


def require_vagrant_environment() -> None:
    common.require_vagrant_environment(VAGRANT_DIR)


def ensure_machine_running(machine: str) -> None:
    common.ensure_machine_running(VAGRANT_DIR, machine)


def get_ssh_host(machine: str) -> Host:
    return common.get_ssh_host(VAGRANT_DIR, machine)


def _run_shell(host: Host, script: str) -> CommandResult:
    quoted = shlex.quote(script)
    return host.run(f"bash -lc {quoted}")


def _sync_work_tree(host: Host) -> None:
    script = f"""
set -euo pipefail
src="{SYNCED_REPO}"
dest="{WORK_TREE}"
if [ ! -d "$src" ]; then
  echo "Shared folder $src is unavailable" >&2
  exit 3
fi
install -d -m 0755 "$dest"
rsync -a --delete "$src"/ "$dest"/
"""
    result = _run_shell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to synchronize xp2p sources on guest.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _install_marker(marker: str, output: str | None) -> str | None:
    for line in (output or "").splitlines():
        line = line.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None


def ensure_xp2p_installed(machine: str, host: Host) -> dict[str, str]:
    cached = _VERSION_CACHE.get(machine)
    if cached:
        return cached

    _sync_work_tree(host)

    script = f"""
set -euo pipefail
export PATH="/usr/local/go/bin:$PATH"
work="{WORK_TREE}"
install_path="{INSTALL_PATH}"
tmpdir=$(mktemp -d)
cleanup() {{
  rm -rf "$tmpdir"
}}
trap cleanup EXIT
cd "$work"
version=$(go run ./go/cmd/xp2p --version | tr -d '\\r')
if [ -z "$version" ]; then
  echo "__XP2P_SOURCE_VERSION__="
  echo "__XP2P_INSTALLED_VERSION__="
  exit 3
fi
ldflags="-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$version"
go build -trimpath -ldflags "$ldflags" -o "$tmpdir/xp2p" ./go/cmd/xp2p
sudo install -m 0755 "$tmpdir/xp2p" "$install_path"
installed=$("$install_path" --version | tr -d '\\r')
echo "__XP2P_SOURCE_VERSION__=$version"
echo "__XP2P_INSTALLED_VERSION__=$installed"
"""
    result = _run_shell(host, script)
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
    _VERSION_CACHE[machine] = versions
    return versions


def run_xp2p(host: Host, *args: str) -> CommandResult:
    quoted_args = " ".join(shlex.quote(arg) for arg in args)
    return host.run(f"{INSTALL_PATH} {quoted_args}")


def machine_host_factory() -> Callable[[str], Host]:
    cache: dict[str, Host] = {}

    def _get(machine: str) -> Host:
        if machine not in MACHINE_IDS:
            raise ValueError(f"Unknown machine id: {machine}")
        if machine not in cache:
            ensure_machine_running(machine)
            cache[machine] = get_ssh_host(machine)
        return cache[machine]

    return _get
