from __future__ import annotations

from pathlib import PurePath
from typing import Iterable

CLIENT_CONFIG_FILES = (
    "inbounds.json",
    "outbounds.json",
    "routing.json",
    "logs.json",
)

SERVER_CONFIG_FILES = (
    "inbounds.json",
    "outbounds.json",
    "routing.json",
    "logs.json",
    "cert.pem",
    "key.pem",
)


def config_paths(config_dir: PurePath, names: Iterable[str]) -> list[PurePath]:
    return [config_dir / name for name in names]
