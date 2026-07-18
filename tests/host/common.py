# Shared helpers for host-side tests (Vagrant, SSH, etc.)
from __future__ import annotations

import shutil
import subprocess
import time
from functools import lru_cache
import functools
import os
from concurrent.futures import ThreadPoolExecutor, TimeoutError as FutureTimeoutError
from pathlib import Path

import pytest
import testinfra
import testinfra.backend
import testinfra.backend.paramiko as paramiko_backend
from testinfra.host import Host

REPO_ROOT = Path(__file__).resolve().parents[2]
SSH_CONNECT_TIMEOUT = 120
SSH_COMMAND_TIMEOUT = 120
VAGRANT_COMMAND_TIMEOUT = 120
SSH_BANNER_TIMEOUT = 10
SSH_AUTH_TIMEOUT = 30
VAGRANT_RELOAD_TIMEOUT = 600


class PatchedParamikoBackend(paramiko_backend.ParamikoBackend):
    @functools.cached_property
    def client(self) -> "paramiko_backend.paramiko.SSHClient":
        client = paramiko_backend.paramiko.SSHClient()
        client.set_missing_host_key_policy(paramiko_backend.paramiko.WarningPolicy())
        connect_timeout = self.timeout
        banner_timeout = min(SSH_BANNER_TIMEOUT, connect_timeout)
        auth_timeout = min(SSH_AUTH_TIMEOUT, connect_timeout)
        cfg = {
            "hostname": self.host.name,
            "port": int(self.host.port) if self.host.port else 22,
            "username": self.host.user,
            "timeout": connect_timeout,
            "banner_timeout": banner_timeout,
            "auth_timeout": auth_timeout,
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
        debug_cfg = {key: value for key, value in cfg.items() if key != "password"}
        start = time.perf_counter()
        print(f"SSH connect start: {debug_cfg}")
        client.connect(**cfg)  # type: ignore[arg-type]
        elapsed = time.perf_counter() - start
        print(f"SSH connect done in {elapsed:.2f}s")
        transport = client.get_transport()
        if transport is not None:
            transport.set_keepalive(30)
        return client

    def _reset_client(self) -> None:
        cache = vars(self)
        cached = cache.get("client")
        if cached is not None:
            try:
                cached.close()
            except Exception:
                pass
        cache.pop("client", None)

    def run(self, command: str, *args: str, **kwargs):  # type: ignore[override]
        kwargs.setdefault("timeout", SSH_COMMAND_TIMEOUT)
        short = command.replace("\r", " ").replace("\n", " ")
        if len(short) > 240:
            short = short[:240] + "..."
        print(f"SSH run start (timeout={kwargs.get('timeout')}): {short}")
        parent_run = super(PatchedParamikoBackend, self).run

        def _run_hard_timeout():
            timeout_value = kwargs.get("timeout") or SSH_COMMAND_TIMEOUT
            with ThreadPoolExecutor(max_workers=1) as executor:
                future = executor.submit(parent_run, command, *args, **kwargs)
                try:
                    return future.result(timeout=float(timeout_value) + 5.0)
                except FutureTimeoutError as exc:
                    self._reset_client()
                    raise paramiko_backend.paramiko.SSHException(
                        f"Guest SSH command hung beyond timeout ({timeout_value}s): {short}"
                    ) from exc
                finally:
                    executor.shutdown(wait=False, cancel_futures=True)
        try:
            result = _run_hard_timeout()
            print(f"SSH run done (rc={result.rc})")
            return result
        except paramiko_backend.paramiko.ssh_exception.NoValidConnectionsError as exc:
            self._reset_client()
            pytest.skip(f"Guest SSH unavailable: {exc}")
        except EOFError:
            self._reset_client()
            return _run_hard_timeout()
        except paramiko_backend.paramiko.SSHException as exc:
            self._reset_client()
            error_text = str(exc).lower()
            if "hung beyond timeout" in error_text:
                raise
            if "no existing session" in error_text or "error reading ssh protocol banner" in error_text:
                max_attempts = 2 if self.timeout <= 30 else 4
                for attempt in range(1, max_attempts + 1):
                    time.sleep(1)
                    try:
                        return _run_hard_timeout()
                    except paramiko_backend.paramiko.SSHException:
                        self._reset_client()
                        if attempt == max_attempts:
                            print("WARNING: SSH banner retry limit reached.")
            try:
                return _run_hard_timeout()
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
                return _run_hard_timeout()
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
def _terminate_process_tree(proc: subprocess.Popen[str]) -> None:
    if proc.poll() is not None:
        return
    if os.name == "nt":
        subprocess.run(
            ["taskkill", "/PID", str(proc.pid), "/T", "/F"],
            capture_output=True,
            text=True,
            check=False,
        )
    else:
        proc.kill()


def _run_vagrant_command(command: list[str], *, cwd: Path, timeout: int) -> str:
    print(f"Vagrant command start (timeout={timeout}s): {' '.join(command)}")
    proc = subprocess.Popen(
        command,
        cwd=cwd,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    try:
        stdout, stderr = proc.communicate(timeout=timeout)
    except subprocess.TimeoutExpired as exc:
        _terminate_process_tree(proc)
        raise subprocess.TimeoutExpired(command, timeout) from exc

    if proc.returncode != 0:
        raise subprocess.CalledProcessError(proc.returncode, command, output=stdout, stderr=stderr)
    print("Vagrant command done")
    return stdout


def machine_state(vagrant_dir: Path, machine: str) -> str | None:
    start = time.perf_counter()
    output = _run_vagrant_command(
        ["vagrant", "status", machine, "--machine-readable"],
        cwd=vagrant_dir,
        timeout=VAGRANT_COMMAND_TIMEOUT,
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
    identity_files: list[str] = []
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
        if key == "identityfile":
            identity_files.append(value)
        config[key] = value

    if identity_files:
        preferred = next((item for item in identity_files if "rsa" in item.lower()), None)
        if preferred is None:
            preferred = next((item for item in identity_files if "ed25519" in item.lower()), None)
        config["identityfile"] = preferred or identity_files[0]

    required = {"hostname", "user", "port", "identityfile"}
    missing = required.difference(config)
    if missing:
        raise RuntimeError(f"Incomplete ssh-config ({missing}) in output:\n{raw}")
    return config


@lru_cache(maxsize=32)
def _ssh_config(vagrant_dir: Path, machine: str) -> str:
    start = time.perf_counter()
    output = _run_vagrant_command(
        ["vagrant", "ssh-config", machine],
        cwd=vagrant_dir,
        timeout=VAGRANT_COMMAND_TIMEOUT,
    )
    elapsed = time.perf_counter() - start
    print(f"TIMING: vagrant ssh-config {machine}: {elapsed:.2f}s")
    return output


def invalidate_ssh_config_cache() -> None:
    _ssh_config.cache_clear()


def vagrant_reload_force(vagrant_dir: Path, machine: str, *, timeout: int = VAGRANT_RELOAD_TIMEOUT) -> None:
    _run_vagrant_command(
        ["vagrant", "reload", machine, "--force"],
        cwd=vagrant_dir,
        timeout=timeout,
    )


def vagrant_reload_provision(vagrant_dir: Path, machine: str, *, timeout: int = VAGRANT_RELOAD_TIMEOUT) -> None:
    _run_vagrant_command(
        ["vagrant", "reload", "--provision", machine],
        cwd=vagrant_dir,
        timeout=timeout,
    )


def get_ssh_host(vagrant_dir: Path, machine: str, *, connect_timeout: int = SSH_CONNECT_TIMEOUT) -> Host:
    ensure_machine_running(vagrant_dir, machine)
    _patch_paramiko_backend()
    raw = _ssh_config(vagrant_dir, machine)
    config = parse_ssh_config(raw)
    print(
        "SSH config resolved: "
        f"host={config['hostname']} port={config['port']} user={config['user']} "
        f"identityfile={config['identityfile']}"
    )
    return testinfra.get_host(
        f"paramiko://{config['user']}@{config['hostname']}:{config['port']}",
        ssh_identity_file=config["identityfile"],
        timeout=connect_timeout,
    )
