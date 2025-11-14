"""
Compatibility facade that exposes the Windows host helpers (tests.host.win.env)
under the historic ``tests.host._env`` module path.

All attributes are forwarded to ``tests.host.win.env`` to keep existing imports
working while the helpers live in the platform-specific package.
"""

from __future__ import annotations

from typing import Any

from tests.host.win import env as _win_env

__all__ = [name for name in dir(_win_env) if not name.startswith("_")]


def __getattr__(name: str) -> Any:
    return getattr(_win_env, name)
