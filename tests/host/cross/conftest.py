from __future__ import annotations

import uuid

import pytest

from tests.host.win import env as win_env


@pytest.fixture(scope="session")
def xp2p_build_id() -> str:
    return uuid.uuid4().hex


@pytest.fixture(scope="session", autouse=True)
def _configure_msi_build_id(xp2p_build_id: str) -> None:
    win_env.set_msi_build_id(xp2p_build_id)
