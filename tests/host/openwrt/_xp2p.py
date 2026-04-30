from __future__ import annotations

import shlex
import time
from contextlib import contextmanager
import os
from pathlib import Path, PurePosixPath

from testinfra.host import Host

from tests.host.openwrt._fs import _posix, _read_file_safe
from tests.host.openwrt._sh import run_guest_script

TARGET_ENV_VAR = "XP2P_OPENWRT_IPK_TARGET"
DEFAULT_TARGET = "linux-amd64"


def run_xp2p_live(host: Host, *args: str):
    cmd = list(args)
    quoted_args = " ".join(shlex.quote(arg) for arg in cmd)
    command = "/usr/bin/xp2p"
    if quoted_args:
        command = f"{command} {quoted_args}"
    return host.run(command)


def run_xp2p(host: Host, *args: str):
    cmd = list(args)
    pending_targets = {
        ("client", "list"),
        ("client", "forward", "list"),
        ("client", "redirect", "list"),
        ("client", "reverse"),
        ("client", "reverse", "list"),
        ("server", "forward", "list"),
        ("server", "redirect", "list"),
        ("server", "reverse"),
        ("server", "reverse", "list"),
        ("server", "user", "list"),
        ("server", "cert", "state"),
    }
    if "--pending" not in cmd and "-y" not in cmd:
        for target in pending_targets:
            if tuple(cmd[: len(target)]) == target:
                cmd.append("--pending")
                break
    return run_xp2p_live(host, *cmd)


def run_xp2p_with_env(host: Host, env_vars: dict[str, str], *args: str):
    assignments = [f"{key}={value}" for key, value in env_vars.items()]
    return run_guest_script(host, "scripts/openwrt/run_xp2p_with_env.sh", *assignments, "--", *args)


def resolve_target_from_env() -> str:
    return os.environ.get(TARGET_ENV_VAR, DEFAULT_TARGET)


def _stop_xp2p_services(host: Host) -> None:
    for cmd in (
        "xp2p client service stop --quiet",
        "xp2p server service stop --quiet",
        "/etc/init.d/xp2p stop",
        "/etc/init.d/xp2p-client stop",
        "/etc/init.d/xp2p-server stop",
    ):
        host.run(f"{cmd} >/dev/null 2>&1 || true")


def _kill_port_listeners(host: Host, port: str) -> None:
    kill_cmd = (
        "pids=$(netstat -lpn 2>/dev/null | grep ':%s ' | awk '{print $7}' | cut -d/ -f1 | tr -d \"-\" ); "
        "for p in $pids; do [ -n \"$p\" ] && kill -9 \"$p\" >/dev/null 2>&1 || true; done"
    ) % shlex.quote(port)
    host.run(f"sh -c {shlex.quote(kill_cmd)}")


def _netstat_snapshot(host: Host) -> str:
    result = host.run("netstat -lpn 2>/dev/null | egrep '62022|62023|52080|52180|51080|51180' || true")
    return result.stdout or ""


@contextmanager
def xp2p_run_session(
    host: Host,
    role: str,
    install_dir: str | Path | PurePosixPath,
    config_dir: str,
    *,
    extra_args: list[str] | None = None,
    log_path: PurePosixPath | Path | str | None = None,
):
    if role not in {"server", "client"}:
        raise ValueError(f"Unsupported role: {role}")
    install_path = _posix(install_dir)
    _stop_xp2p_services(host)
    for port in ("62022", "62023", "52080", "52180", "51080", "51180"):
        host.run(f"fuser -k {port}/tcp >/dev/null 2>&1 || true")
        host.run(f"fuser -k {port}/udp >/dev/null 2>&1 || true")
        _kill_port_listeners(host, port)
    host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
    host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
    host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
    host.run("ps w | grep '/etc/xp2p/bin/xray' | grep -v grep | awk '{print $1}' | xargs -r kill -9 >/dev/null 2>&1 || true")
    host.run("ps w | grep '/usr/bin/xp2p' | grep -v grep | awk '{print $1}' | xargs -r kill -9 >/dev/null 2>&1 || true")
    netstat_before = _netstat_snapshot(host)
    logs_path = PurePosixPath(install_dir) / config_dir / "logs.json"
    logs_config = _read_file_safe(host, logs_path)
    extra = ""
    if extra_args:
        extra = " " + " ".join(shlex.quote(str(arg)) for arg in extra_args)
    if log_path is None:
        log_target = f"/tmp/xp2p-{role}-run.log"
    elif isinstance(log_path, (PurePosixPath, Path)):
        log_target = log_path.as_posix()
    else:
        log_target = str(log_path)
    start_cmd = (
        f"setsid /usr/bin/xp2p {role} run "
        f"--path {shlex.quote(install_path)} "
        f"--config-dir {shlex.quote(config_dir)} "
        f"--quiet{extra} >{shlex.quote(log_target)} 2>&1 & echo $!"
    )
    last_log = ""
    pid_value: str | None = None
    for attempt in range(2):
        result = host.run(f"sh -c {shlex.quote(start_cmd)}")
        if result.rc != 0:
            raise RuntimeError(
                f"Failed to start xp2p {role} run.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        pid_line = (result.stdout or "").strip().splitlines()
        if not pid_line:
            raise RuntimeError("xp2p run did not output PID")
        pid_value = pid_line[-1].strip()
        time.sleep(1)
        alive = host.run(f"kill -0 {pid_value} >/dev/null 2>&1")
        if alive.rc == 0:
            break
        log_read = host.run(f"cat {shlex.quote(log_target)} 2>/dev/null || true")
        last_log = log_read.stdout or ""
        host.run(f"pkill -f 'xp2p {role} run' >/dev/null 2>&1 || true")
        host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
        time.sleep(1.0)
    else:
        raise RuntimeError(
            "xp2p {role} run exited prematurely (pid {pid}).\n"
            "Log output:\n{log}\n"
            "netstat before start:\n{netstat}\n"
            "logs.json ({logs_path}):\n{logs_cfg}".format(
                role=role,
                pid=pid_value,
                log=last_log,
                netstat=netstat_before,
                logs_path=logs_path.as_posix(),
                logs_cfg=logs_config,
            )
        )
    try:
        yield {"pid": int(pid_value)}
    finally:
        host.run(f"kill {pid_value} >/dev/null 2>&1 || true")
