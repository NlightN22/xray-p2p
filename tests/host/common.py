# Shared helpers for host-side tests (Vagrant, SSH, etc.)
from __future__ import annotations

import shutil
import subprocess
from functools import lru_cache
from pathlib import Path

import pytest
import testinfra
from testinfra.host import Host

REPO_ROOT = Path(__file__).resolve().parents[2]


def require_vagrant_environment(vagrant_dir: Path) -> None:
    if shutil.which("vagrant") is None:
        pytest.skip("Vagrant executable not found on host; guest tests are unavailable.")
    if not vagrant_dir.exists():
        pytest.skip(
            f"Expected Vagrant environment at '{vagrant_dir}' is missing; "
            "run the appropriate `make vagrant-*` target before invoking host tests."
        )


def ensure_machine_running(vagrant_dir: Path, machine: str) -> None:
    try:
        state = machine_state(vagrant_dir, machine)
    except subprocess.CalledProcessError as exc:
        pytest.skip(
            f"Unable to determine state for guest '{machine}' "
            f"(vagrant status exited with code {exc.returncode}). "
            "Run the corresponding `make vagrant-*` target and retry."
        )
    if state != "running":
        pytest.skip(
            f"Guest '{machine}' is not running (state={state!r}). "
            "Start the VM via `make vagrant-*` and retry."
        )


@lru_cache(maxsize=32)
def machine_state(vagrant_dir: Path, machine: str) -> str | None:
    output = subprocess.check_output(
        ["vagrant", "status", machine, "--machine-readable"],
        cwd=vagrant_dir,
        text=True,
    )
    for line in output.splitlines():
        parts = line.split(",")
        if len(parts) >= 4 and parts[2] == "state":
            return parts[3]
    return None


def parse_ssh_config(raw: str) -> dict[str, str]:
    config: dict[str, str] = {}
    for line in raw.splitlines():
        line = line.strip()
        if not line or line.lower().startswith("host "):
            continue
        pieces = line.split(None, 1)
        if len(pieces) != 2:
            continue
        key = pieces[0].lower()
        value = pieces[1].strip()
        if value.startswith('"') and value.endswith('"'):
            value = value[1:-1]
        config[key] = value

    required = {"hostname", "user", "port", "identityfile"}
    missing = required.difference(config)
    if missing:
        raise RuntimeError(f"Incomplete ssh-config ({missing}) in output:\n{raw}")
    return config


@lru_cache(maxsize=32)
def _ssh_config(vagrant_dir: Path, machine: str) -> str:
    return subprocess.check_output(
        ["vagrant", "ssh-config", machine],
        cwd=vagrant_dir,
        text=True,
    )


def get_ssh_host(vagrant_dir: Path, machine: str) -> Host:
    ensure_machine_running(vagrant_dir, machine)
    raw = _ssh_config(vagrant_dir, machine)
    config = parse_ssh_config(raw)
    return testinfra.get_host(
        f"paramiko://{config['user']}@{config['hostname']}:{config['port']}",
        ssh_identity_file=config["identityfile"],
    )
