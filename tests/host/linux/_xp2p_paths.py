from __future__ import annotations

from pathlib import PurePosixPath

WORK_TREE = PurePosixPath("/srv/xray-p2p")
INSTALL_PATH = PurePosixPath("/usr/bin/xp2p")
GUEST_SCRIPTS_ROOT = WORK_TREE / "tests" / "guest"

