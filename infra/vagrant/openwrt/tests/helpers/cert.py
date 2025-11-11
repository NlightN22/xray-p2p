from __future__ import annotations

import shlex
from typing import Dict

from .scripts import (
    server_cert_apply_script_path,
    server_cert_paths_script_path,
    server_cert_selfsigned_script_path,
    common_loader_script_path,
    common_script_path,
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
    host.run(
        "rm -f /tmp/server_install_cert_apply.sh /tmp/server_cert_paths.sh /tmp/common_loader.sh /tmp/common.sh"
    )

    script_path = server_cert_apply_script_path(host)
    paths_path = server_cert_paths_script_path(host)
    loader_path = common_loader_script_path(host)
    common_path = common_script_path(host)

    staged_dir = "/tmp/scripts/lib"
    run_checked(host, f"mkdir -p {shlex.quote(staged_dir)}", "prepare cert helper directory")
    run_checked(
        host,
        f"cp {shlex.quote(paths_path)} {shlex.quote(staged_dir)}/server_cert_paths.sh",
        "stage server_cert_paths helper",
    )
    run_checked(
        host,
        f"cp {shlex.quote(loader_path)} {shlex.quote(staged_dir)}/common_loader.sh",
        "stage common loader",
    )
    run_checked(
        host,
        f"cp {shlex.quote(common_path)} {shlex.quote(staged_dir)}/common.sh",
        "stage common library",
    )

    script = shlex.quote(script_path)
    effective_env: Dict[str, str] = dict(env or {})
    effective_env.setdefault("XRAY_SERVER_CERT_PATHS_LIB", f"{staged_dir}/server_cert_paths.sh")
    effective_env.setdefault("XRAY_SELF_DIR", "/tmp")
    effective_env.setdefault("XRAY_SCRIPT_ROOT", "/tmp")
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
