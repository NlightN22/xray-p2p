from __future__ import annotations

import shlex
from pathlib import Path
from typing import Callable

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from tests.host import common

REPO_ROOT = common.REPO_ROOT
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "debian12" / "deb-test"
MACHINE_IDS: tuple[str, ...] = (
    "deb-test-a",
    "deb-test-b",
    "deb-test-c",
)
WORK_TREE = Path("/srv/xray-p2p")
INSTALL_PATH = Path("/usr/bin/xp2p")
BUILD_SCRIPT = WORK_TREE / "scripts" / "build" / "build_deb_xp2p.sh"
ARTIFACT_DIR = WORK_TREE / "build" / "deb" / "artifacts"

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

    script = f"""
set -euo pipefail
export PATH="/usr/local/go/bin:$PATH"
work="{WORK_TREE}"
install_path="{INSTALL_PATH}"
build_script="{BUILD_SCRIPT}"
artifact_dir="{ARTIFACT_DIR}"
if [ ! -d "$work" ]; then
  echo "__XP2P_SOURCE_VERSION__="
  echo "__XP2P_INSTALLED_VERSION__="
  echo "Missing xp2p repo at $work" >&2
  exit 3
fi
if [ ! -x "$build_script" ]; then
  echo "__XP2P_SOURCE_VERSION__="
  echo "__XP2P_INSTALLED_VERSION__="
  echo "Build script $build_script is not executable" >&2
  exit 3
fi
cd "$work"
source_version=$(go run ./go/cmd/xp2p --version | tr -d '\\r')
if [ -z "$source_version" ]; then
  echo "__XP2P_SOURCE_VERSION__="
  echo "__XP2P_INSTALLED_VERSION__="
  exit 3
fi
installed_version=""
need_install=0
if [ -x "$install_path" ]; then
  installed_version=$("$install_path" --version | tr -d '\\r')
  if [ "$installed_version" != "$source_version" ]; then
    need_install=1
  fi
else
  need_install=1
fi
if [ "$need_install" -eq 1 ]; then
  "$build_script"
  shopt -s nullglob
  arch=$(dpkg --print-architecture)
  latest_pkg=""
  for pkg in "$artifact_dir"/xp2p_*_"$arch".deb; do
    if [ -z "$latest_pkg" ] || [ "$pkg" -nt "$latest_pkg" ]; then
      latest_pkg="$pkg"
    fi
  done
  shopt -u nullglob
  if [ -z "$latest_pkg" ]; then
    echo "__XP2P_SOURCE_VERSION__="
    echo "__XP2P_INSTALLED_VERSION__="
    echo "xp2p package not found in $artifact_dir" >&2
    exit 3
  fi
  sudo dpkg -i "$latest_pkg"
  installed_version=$("$install_path" --version | tr -d '\\r')
fi
echo "__XP2P_SOURCE_VERSION__=$source_version"
echo "__XP2P_INSTALLED_VERSION__=$installed_version"
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
