from __future__ import annotations

import shlex
import time

from testinfra.host import Host


def stop_process(host: Host, pid: int | str) -> None:
    pid_arg = shlex.quote(str(pid))
    script = (
        "pid=\"$1\"; "
        "case \"$pid\" in ''|*[!0-9]*) exit 0;; esac; "
        "kill -0 \"$pid\" >/dev/null 2>&1 || exit 0; "
        "kill \"$pid\" >/dev/null 2>&1 || true; "
        "i=0; "
        "while [ $i -lt 20 ]; do "
        "kill -0 \"$pid\" >/dev/null 2>&1 || exit 0; "
        "sleep 1; "
        "i=$((i+1)); "
        "done; "
        "kill -9 \"$pid\" >/dev/null 2>&1 || true"
    )
    host.run(f"sudo -n /bin/sh -c {shlex.quote(script)} -- {pid_arg}")


def kill_xp2p_processes(host: Host) -> None:
    patterns = [
        "xp2p client run",
        "xp2p server run",
        "xp2p client deploy",
        "xp2p server deploy",
        "/etc/xp2p/bin/xray",
        "/usr/bin/xp2p",
    ]
    pgrep_patterns = [
        "[x]p2p client run",
        "[x]p2p server run",
        "[x]p2p client deploy",
        "[x]p2p server deploy",
        "/etc/xp2p/bin/[x]ray",
        "/usr/bin/[x]p2p",
    ]
    for pattern in patterns:
        quoted = shlex.quote(pattern)
        host.run(
            "sudo -n /bin/sh -c "
            "'pkill -f \"$1\" >/dev/null 2>&1 || true' "
            f"-- {quoted}"
        )

    for _ in range(10):
        running = []
        for pattern in pgrep_patterns:
            quoted = shlex.quote(pattern)
            result = host.run(
                "sudo -n /bin/sh -c "
                "'pgrep -f \"$1\" >/dev/null 2>&1' "
                f"-- {quoted}"
            )
            if result.rc == 0:
                running.append(pattern)
        if not running:
            return
        time.sleep(1)

    details = ["xp2p process cleanup failed"]
    for pattern in running:
        quoted = shlex.quote(pattern)
        result = host.run(
            "sudo -n /bin/sh -c "
            "'pgrep -af \"$1\" || true' "
            f"-- {quoted}"
        )
        if result.stdout:
            details.append(f"still running: {pattern}\n{result.stdout.strip()}")
        else:
            details.append(f"still running: {pattern}")
    raise RuntimeError("\n".join(details))

