import subprocess
from pathlib import Path
from urllib.parse import urlencode

import pytest
import testinfra


def _parse_vagrant_ssh_config(raw: str) -> dict[str, str]:
    config: dict[str, str] = {}
    for line in raw.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue

        parts = line.split(None, 1)
        if len(parts) != 2:
            continue

        key = parts[0].lower()
        value = parts[1].strip().strip('"')

        if key == "identityfile" and "identityfile" in config:
            continue

        config[key] = value

    required = {"hostname", "user", "identityfile", "port"}
    missing = required.difference(config)
    if missing:
        raise RuntimeError(f"Incomplete ssh-config ({missing}) in output: {raw}")

    return config


def _connection_from_config(config: dict[str, str]) -> str:
    identity = Path(config["identityfile"]).expanduser()
    query = urlencode(
        {
            "ssh_identity_file": str(identity),
            "load_known_hosts": "true",
            "disabled_algorithms": "hostkey=ssh-ed25519",
        }
    )
    return f"paramiko://{config['user']}@{config['hostname']}:{config['port']}?{query}"


@pytest.fixture(scope="session")
def vagrant_dir() -> Path:
    return Path(__file__).resolve().parent.parent


@pytest.fixture(scope="session")
def vagrant_host_factory(vagrant_dir: Path):
    def _factory(machine: str):
        raw = subprocess.check_output(
            ["vagrant", "ssh-config", machine],
            cwd=vagrant_dir,
            text=True,
        )
        config = _parse_vagrant_ssh_config(raw)
        return testinfra.get_host(_connection_from_config(config))

    return _factory


@pytest.fixture(scope="session")
def host_r1(vagrant_host_factory):
    return vagrant_host_factory("r1")


@pytest.fixture(scope="session")
def host_r2(vagrant_host_factory):
    return vagrant_host_factory("r2")


@pytest.fixture(scope="session")
def host_r3(vagrant_host_factory):
    return vagrant_host_factory("r3")


@pytest.fixture(scope="session")
def host_c1(vagrant_host_factory):
    return vagrant_host_factory("c1")


@pytest.fixture(scope="session")
def host_c2(vagrant_host_factory):
    return vagrant_host_factory("c2")


@pytest.fixture(scope="session")
def host_c3(vagrant_host_factory):
    return vagrant_host_factory("c3")


def pytest_collection_modifyitems(session, config, items):
    def sort_key(item):
        path = item.fspath.basename
        if path.startswith("test_stage"):
            return (0, path, item.name)
        return (1, path, item.name)

    items.sort(key=sort_key)
