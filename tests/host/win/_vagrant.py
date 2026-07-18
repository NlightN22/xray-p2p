import time
import subprocess

import pytest
from testinfra.host import Host

from tests.host import common


def available_win_stacks() -> list[str]:
    from . import env as _env

    return sorted(_env.WIN_STACKS.keys())


def set_win_stack(name: str) -> None:
    from . import env as _env

    if name not in _env.WIN_STACKS:
        raise ValueError(
            f"Unknown win stack '{name}'. Available: {', '.join(available_win_stacks())}"
        )
    config = _env.WIN_STACKS[name]
    _env._CURRENT_WIN_STACK = name
    _env.VAGRANT_DIR = config["vagrant_dir"]
    _env.DEFAULT_SERVER = config["server"]
    _env.DEFAULT_CLIENT = config["client"]
    _env.DEFAULT_TARGET = config["target"]


def require_vagrant_environment() -> None:
    from . import env as _env

    common.require_vagrant_environment(_env.VAGRANT_DIR)


def ensure_machine_running(machine: str) -> None:
    from . import env as _env

    common.ensure_machine_running(_env.VAGRANT_DIR, machine)


def get_ssh_host(machine: str) -> Host:
    from . import env as _env

    probe_timeout = 30

    def _probe(host: Host) -> None:
        result = _env.run_powershell(
            host,
            "[Environment]::MachineName; whoami",
            timeout=probe_timeout,
            label="ssh_probe",
        )
        if result.rc != 0:
            raise RuntimeError(
                "SSH probe command failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

    last_exc: BaseException | None = None
    for attempt in range(2):
        try:
            host = common.get_ssh_host(_env.VAGRANT_DIR, machine, connect_timeout=30)
            setattr(host, "_xp2p_machine", machine)
            _probe(host)
            return host
        except pytest.skip.Exception as exc:
            last_exc = exc
            if attempt > 0:
                break
            print(
                f"WARNING: Failed to probe guest {machine} over SSH: {exc}. "
                "Reloading the VM and retrying once."
            )
            try:
                common.vagrant_reload_force(_env.VAGRANT_DIR, machine)
            except subprocess.CalledProcessError as reload_exc:
                raise RuntimeError(f"Vagrant reload failed for {machine}: {reload_exc}") from reload_exc
            finally:
                common.invalidate_ssh_config_cache()
            time.sleep(5)
            continue
        except BaseException as exc:
            last_exc = exc
            if attempt > 0:
                break
            print(
                f"WARNING: Failed to probe guest {machine} over SSH: {exc}. "
                "Reloading the VM and retrying once."
            )
            try:
                common.vagrant_reload_force(_env.VAGRANT_DIR, machine)
            except subprocess.CalledProcessError as reload_exc:
                raise RuntimeError(f"Vagrant reload failed for {machine}: {reload_exc}") from reload_exc
            finally:
                common.invalidate_ssh_config_cache()
            time.sleep(5)

    if last_exc is None:
        raise RuntimeError(f"Failed to connect to guest {machine} via SSH.")
    raise last_exc
