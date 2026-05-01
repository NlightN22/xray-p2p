from __future__ import annotations

from collections.abc import Callable

from testinfra.host import Host

from tests.host import common

REPO_ROOT = common.REPO_ROOT
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "debian12" / "deb-test"
MACHINE_IDS: tuple[str, ...] = (
    "deb-test-a",
    "deb-test-b",
    "deb-test-c",
)
DEFAULT_CLIENT = MACHINE_IDS[0]
DEFAULT_SERVER = MACHINE_IDS[1]
DEFAULT_AUX = MACHINE_IDS[2]


def require_vagrant_environment() -> None:
    common.require_vagrant_environment(VAGRANT_DIR)


def ensure_machine_running(machine: str) -> None:
    common.ensure_machine_running(VAGRANT_DIR, machine)


def get_ssh_host(machine: str) -> Host:
    return common.get_ssh_host(VAGRANT_DIR, machine)


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

