"""
Compatibility shim that exposes `_env` under `tests.host.win.win`.

The actual helpers live in ``tests.host.win.env`` (with shared helpers in
``tests.host.common``), but legacy imports expect
``from tests.host.win.win import _env``. Keep that interface stable by
re-exporting the module.
"""

from . import env as shared_env

_env = shared_env

__all__ = ["_env"]
