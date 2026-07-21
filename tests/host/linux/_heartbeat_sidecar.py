from __future__ import annotations

import base64
from contextlib import contextmanager
import json
import shlex

from tests.host.host_common.polling import wait_until
from tests.host.linux import _bare as bare


@contextmanager
def late_sidecar(server_host, protocol: str):
    root = "/tmp/xp2p-late-sidecar"
    user = f"bare-{protocol}"
    credential = bare.TROJAN_PASSWORD if protocol == "trojan" else bare.VLESS_UUID
    runtime = {
        "control": {
            "subscription": {"generation": "late-sidecar"},
            "auth_users": [{"label": user, "credential": credential}],
        }
    }
    encoded = base64.b64encode(json.dumps(runtime).encode()).decode()
    command = (
        f"rm -rf {root}; mkdir -p {root}/.state/live/config-server {root}/install; "
        f"echo {shlex.quote(encoded)} | base64 -d > {root}/.state/live/config-server/runtime.json; "
        "ip addr add 198.18.0.2/32 dev lo 2>/dev/null || true; "
        f"XP2P_CONFIG_ROOT={root} XP2P_SERVER_INSTALL_DIR={root}/install "
        f"XP2P_SERVER_CERTIFICATE={bare.CERT} XP2P_SERVER_KEY={bare.KEY} "
        f"/usr/bin/xp2p diag --listen 0.0.0.0:62022 >{root}/sidecar.log 2>&1 & "
        f"echo $! > {root}/sidecar.pid"
    )
    started = server_host.run(f"sudo -n /bin/sh -c {shlex.quote(command)}")
    assert started.rc == 0, started.stderr
    try:
        wait_until(
            "late diagnostics sidecar",
            lambda: True
            if server_host.run("ss -ltn | grep -q ':62022 '").rc == 0
            else None,
            timeout_seconds=15.0,
            poll_interval=0.5,
        )
        yield
    finally:
        server_host.run(
            f"sudo -n /bin/sh -c 'kill $(cat {root}/sidecar.pid) 2>/dev/null || true; "
            f"ip addr del 198.18.0.2/32 dev lo 2>/dev/null || true; rm -rf {root}'"
        )
