import time
from collections.abc import Callable

from testinfra.host import Host

from tests.host import common


def ensure_project_synced(
    host: Host,
    *,
    timeout: int = 60,
    machine: str | None = None,
    reconnect: Callable[[], Host] | None = None,
) -> Host:
    from . import env as _env

    if _wait_for_sync_marker(host, timeout=timeout):
        return host

    if machine and reconnect is not None:
        reload_steps: list[tuple[str, Callable]] = [
            ("reload --force", common.vagrant_reload_force),
            ("reload --provision", common.vagrant_reload_provision),
        ]
        for step_label, reload_fn in reload_steps:
            print(
                f"WARNING: Project sync marker missing on guest {machine}. "
                f"Attempting '{step_label}' once to re-mount synced folder."
            )
            try:
                reload_fn(_env.VAGRANT_DIR, machine)
            finally:
                common.invalidate_ssh_config_cache()
            host = reconnect()
            if _wait_for_sync_marker(host, timeout=timeout):
                return host

    if machine:
        print(
            f"WARNING: Project sync marker missing on guest {machine}. "
            "Reloading with provision once to re-mount synced folder."
        )
        try:
            common.vagrant_reload_provision(_env.VAGRANT_DIR, machine)
        finally:
            common.invalidate_ssh_config_cache()
            backend = getattr(host, "backend", None)
            if backend is not None and hasattr(backend, "_reset_client"):
                backend._reset_client()

        if _wait_for_sync_marker(host, timeout=timeout):
            return host

    hint = ""
    if machine:
        hint = f" Re-mount the synced folder (try 'vagrant reload --provision {machine}')."
    raise RuntimeError(
        "Project sync not ready on guest. "
        f"Expected {_env.PROJECT_SYNC_MARKER} to exist.{hint}"
    )


def _wait_for_sync_marker(host: Host, *, timeout: int) -> bool:
    from . import env as _env

    start = time.monotonic()
    while time.monotonic() - start < timeout:
        if _env.path_exists(host, _env.PROJECT_SYNC_MARKER):
            return True
        time.sleep(2)
    return False

