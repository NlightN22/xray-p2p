"""
Deprecated shim for historical imports.

Windows-specific helpers now live in ``tests.host.win.env``.
Importing from ``tests.host._env`` will continue to work, but new code should
use ``tests.host.win.env`` (and ``tests.host.common`` for shared utilities).
"""

from tests.host.win.env import *  # noqa: F401,F403
