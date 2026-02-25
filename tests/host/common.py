# Shared helpers for host-side tests (Vagrant, SSH, etc.)
from __future__ import annotations

import shutil
import subprocess
import time
from functools import lru_cache
import functools
import os
from pathlib import Path

import pytest
import testinfra
import testinfra.backend
import testinfra.backend.paramiko as paramiko_backend
from testinfra.host import Host

REPO_ROOT = Path(__file__).resolve().parents[2]
SSH_CONNECT_TIMEOUT = 15
SSH_COMMAND_TIMEOUT = 120


class PatchedParamikoBackend(paramiko_backend.ParamikoBackend):
    @functools.cached_property
    def client(self) -> "paramiko_backend.paramiko.SSHClient":
        client = paramiko_backend.paramiko.SSHClient()
        client.set_missing_host_key_policy(paramiko_backend.paramiko.WarningPolicy())
        cfg = {
            "hostname": self.host.name,
            "port": int(self.host.port) if self.host.port else 22,
            "username": self.host.user,
            "timeout": self.timeout,
            "password": self.host.password,
            "look_for_keys": False,
            "allow_agent": False,
        }
        if self.ssh_config:
            ssh_config_dir = os.path.dirname(self.ssh_config)

            with open(self.ssh_config) as f:
                ssh_config = paramiko_backend.paramiko.SSHConfig()
                ssh_config.parse(f)
                self._load_ssh_config(client, cfg, ssh_config, ssh_config_dir)
        else:
            default_ssh_config = os.path.join(os.path.expanduser("~"), ".ssh", "config")
            ssh_config_dir = os.path.dirname(default_ssh_config)

            try:
                with open(default_ssh_config) as f:
                    ssh_config = paramiko_backend.paramiko.SSHConfig()
                    ssh_config.parse(f)
            except OSError:
                pass
            else:
                self._load_ssh_config(client, cfg, ssh_config, ssh_config_dir)

        if self.ssh_identity_file:
            cfg["key_filename"] = self.ssh_identity_file
        client.connect(**cfg)  # type: ignore[arg-type]
        transport = client.get_transport()
        if transport is not None:
            transport.set_keepalive(30)
        return client

    def _reset_client(self) -> None:
        cached = self.__dict__.get("client")
        if cached is not None:
            try:
                cached.close()
            except Exception:
                pass
        self.__dict__.pop("client", None)

    def run(self, command: str, *args: str, **kwargs):  # type: ignore[override]
        kwargs.setdefault("timeout", SSH_COMMAND_TIMEOUT)
        try:
            return super().run(command, *args, **kwargs)
        except paramiko_backend.paramiko.ssh_exception.NoValidConnectionsError as exc:
            self._reset_client()
            pytest.skip(f"Guest SSH unavailable: {exc}")
        except EOFError:
            self._reset_client()
            return super().run(command, *args, **kwargs)
        except paramiko_backend.paramiko.SSHException as exc:
            self._reset_client()
            try:
                return super().run(command, *args, **kwargs)
            except paramiko_backend.paramiko.ssh_exception.NoValidConnectionsError as exc:
                self._reset_client()
                pytest.skip(f"Guest SSH unavailable: {exc}")
            except paramiko_backend.paramiko.SSHException as exc_retry:
                self._reset_client()
                pytest.skip(f"Guest SSH unavailable: {exc_retry}")
        except OSError as exc:
            winerror = getattr(exc, "winerror", None)
            err_no = getattr(exc, "errno", None)
            if winerror not in (10054,) and err_no not in (104,):
                raise
            self._reset_client()
            try:
                return super().run(command, *args, **kwargs)
            except paramiko_backend.paramiko.ssh_exception.NoValidConnectionsError as exc:
                self._reset_client()
                pytest.skip(f"Guest SSH unavailable: {exc}")
            except paramiko_backend.paramiko.SSHException as exc_retry:
                self._reset_client()
                pytest.skip(f"Guest SSH unavailable: {exc_retry}")


def _patch_paramiko_backend() -> None:
    if testinfra.backend.BACKENDS.get("paramiko") == "tests.host.common.PatchedParamikoBackend":
        return
    testinfra.backend.BACKENDS["paramiko"] = "tests.host.common.PatchedParamikoBackend"


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
    start = time.perf_counter()
    output = subprocess.check_output(
        ["vagrant", "status", machine, "--machine-readable"],
        cwd=vagrant_dir,
        text=True,
    )
    elapsed = time.perf_counter() - start
    print(f"TIMING: vagrant status {machine}: {elapsed:.2f}s")
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
    start = time.perf_counter()
    output = subprocess.check_output(
        ["vagrant", "ssh-config", machine],
        cwd=vagrant_dir,
        text=True,
    )
    elapsed = time.perf_counter() - start
    print(f"TIMING: vagrant ssh-config {machine}: {elapsed:.2f}s")
    return output


def get_ssh_host(vagrant_dir: Path, machine: str) -> Host:
    ensure_machine_running(vagrant_dir, machine)
    _patch_paramiko_backend()
    raw = _ssh_config(vagrant_dir, machine)
    config = parse_ssh_config(raw)
    return testinfra.get_host(
        f"paramiko://{config['user']}@{config['hostname']}:{config['port']}",
        ssh_identity_file=config["identityfile"],
        timeout=SSH_CONNECT_TIMEOUT,
    )
