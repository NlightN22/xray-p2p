from __future__ import annotations

import shlex
from typing import Dict

from .scripts import (
    server_cert_apply_script_path,
    server_cert_paths_script_path,
    server_cert_selfsigned_script_path,
)
from .utils import run_checked


def server_cert_apply(
    host,
    cert_path: str,
    key_path: str,
    *,
    inbounds_path: str | None = None,
    env: Dict[str, str] | None = None,
    check: bool = True,
    description: str | None = None,
):
    script_path = server_cert_apply_script_path(host)
    paths_path = server_cert_paths_script_path(host)

    staged_dir = "/tmp/scripts/lib"
    run_checked(host, f"mkdir -p {shlex.quote(staged_dir)}", "prepare cert helper directory")
    run_checked(
        host,
        f"cp {shlex.quote(paths_path)} {shlex.quote(staged_dir)}/server_cert_paths.sh",
        "stage server_cert_paths helper",
    )
    verify_cmd = (
        "sh -c "
        + shlex.quote(
            f"set -e;"
            f". {staged_dir}/server_cert_paths.sh;"
            "command -v server_cert_paths_update >/dev/null"
        )
    )
    run_checked(host, verify_cmd, "verify server_cert_paths helper availability")

    script = shlex.quote(script_path)
    effective_env: Dict[str, str] = dict(env or {})
    effective_env.setdefault(
        "XRAY_SERVER_CERT_PATHS_LIB",
        f"{staged_dir}/server_cert_paths.sh",
    )
    tokens = [
        script,
        "--cert",
        shlex.quote(cert_path),
        "--key",
        shlex.quote(key_path),
    ]
    if inbounds_path:
        tokens.extend(["--inbounds", shlex.quote(inbounds_path)])
    cmd = " ".join(tokens)
    if effective_env:
        exports = " ".join(
            f"{key}={shlex.quote(str(value))}"
            for key, value in effective_env.items()
            if value is not None
        )
        if exports:
            cmd = f"{exports} {cmd}"
    if check:
        return run_checked(host, cmd, description or "apply server certificate")
    return host.run(cmd)


def server_cert_selfsigned(
    host,
    *,
    inbounds_path: str | None = None,
    env: Dict[str, str] | None = None,
    check: bool = True,
    description: str | None = None,
):
    script = shlex.quote(server_cert_selfsigned_script_path(host))
    tokens = [script]
    if inbounds_path:
        tokens.extend(["--inbounds", shlex.quote(inbounds_path)])
    cmd = " ".join(tokens)
    if env:
        exports = " ".join(
            f"{key}={shlex.quote(str(value))}"
            for key, value in env.items()
            if value is not None
        )
        if exports:
            cmd = f"{exports} {cmd}"
    if check:
        return run_checked(
            host,
            cmd,
            description or "generate self-signed certificate",
        )
    return host.run(cmd)


__all__ = ["server_cert_apply", "server_cert_selfsigned"]
