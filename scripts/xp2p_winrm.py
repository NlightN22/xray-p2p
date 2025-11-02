#!/usr/bin/env python3
from __future__ import annotations

import argparse
import base64
import re
import subprocess
import sys
import threading
from pathlib import Path
from typing import Iterable


def _repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def _ps_literal(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _format_display(args: Iterable[str]) -> str:
    parts: list[str] = ["xp2p.exe"]
    for arg in args:
        if re.search(r"""[\s"'`^]""", arg):
            parts.append('"' + arg.replace('"', '\\"') + '"')
        else:
            parts.append(arg)
    return " ".join(parts)


def _find_vm_directory(repo_root: Path, vm_name: str) -> Path:
    candidates = [
        path
        for path in (repo_root / "infra" / "vagrant-win").iterdir()
        if (path / "Vagrantfile").is_file()
    ]
    matches: list[Path] = []
    for candidate in candidates:
        result = subprocess.run(
            ["vagrant", "status", vm_name, "--machine-readable"],
            cwd=candidate,
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        if result.returncode == 0:
            matches.append(candidate)
    if not matches:
        raise FileNotFoundError(f"Unable to find a Vagrant environment containing '{vm_name}'.")
    if len(matches) > 1:
        joined = ", ".join(str(match) for match in matches)
        raise RuntimeError(f"Multiple Vagrant environments match '{vm_name}': {joined}")
    return matches[0]


def _run(command: list[str], *, cwd: Path | None = None) -> None:
    process = subprocess.run(command, cwd=cwd, check=False)
    if process.returncode != 0:
        raise RuntimeError(
            f"Command {' '.join(command)} exited with code {process.returncode}."
        )


def _run_stream(command: list[str], *, cwd: Path | None = None) -> tuple[int, str, str]:
    process = subprocess.Popen(
        command,
        cwd=cwd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
    )

    if process.stdout is None or process.stderr is None:
        raise RuntimeError("Failed to capture process output.")

    stdout_buffer: list[str] = []
    stderr_buffer: list[str] = []

    def _reader(pipe, printer, buffer):
        with pipe:
            for line in iter(pipe.readline, ""):
                printer(line)
                buffer.append(line)

    threads = [
        threading.Thread(
            target=_reader,
            args=(process.stdout, lambda s: print(s, end="", flush=True), stdout_buffer),
            daemon=True,
        ),
        threading.Thread(
            target=_reader,
            args=(process.stderr, lambda s: print(s, end="", file=sys.stderr, flush=True), stderr_buffer),
            daemon=True,
        ),
    ]

    for thread in threads:
        thread.start()

    returncode = process.wait()

    for thread in threads:
        thread.join()

    return returncode, "".join(stdout_buffer), "".join(stderr_buffer)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Build xp2p and execute it on a Windows guest via Vagrant WinRM."
    )
    parser.add_argument(
        "--vm",
        default="win10-server",
        help="Vagrant VM name to target (default: win10-server).",
    )
    parser.add_argument(
        "--skip-build",
        action="store_true",
        help="Skip running `make build-windows` (use existing binary).",
    )
    parser.add_argument(
        "xp2p_args",
        nargs=argparse.REMAINDER,
        help="Arguments to pass to xp2p.exe. Separate them with `--`, e.g. -- ping 127.0.0.1 --port 62022",
    )

    args = parser.parse_args(argv)

    xp2p_arguments = list(args.xp2p_args)
    if xp2p_arguments and xp2p_arguments[0] == "--":
        xp2p_arguments = xp2p_arguments[1:]

    repo_root = _repo_root()
    print(f"==> Repository root: {repo_root}")

    if not args.skip_build:
        print("==> Building xp2p via `make build-windows`")
        _run(["make", "build-windows"], cwd=repo_root)

    exe_path = repo_root / "build" / "windows-amd64" / "xp2p.exe"
    if not exe_path.is_file():
        raise FileNotFoundError(f"xp2p.exe not found at {exe_path}")
    print(f"==> Local binary: {exe_path}")

    vm_dir = _find_vm_directory(repo_root, args.vm)
    print(f"==> Using Vagrant environment: {vm_dir}")

    print(f"==> Running `vagrant up {args.vm} --no-provision`")
    _run(["vagrant", "up", args.vm, "--no-provision"], cwd=vm_dir)

    display_command = _format_display(xp2p_arguments)
    print(f"==> Effective command: {display_command}")

    remote_exe = r"C:\xp2p\build\windows-amd64\xp2p.exe"
    ps_arguments = ", ".join(_ps_literal(arg) for arg in xp2p_arguments)
    remote_script = f"""
$ErrorActionPreference = 'Stop'
$exe = {_ps_literal(remote_exe)}
if (-not (Test-Path $exe)) {{
    throw "xp2p executable not found at $exe"
}}
$info = Get-Item $exe
Write-Host ("==> Guest binary: " + $info.FullName + " (LastWriteTime: " + $info.LastWriteTime.ToString("u") + ")")
$argsList = @({ps_arguments})
Write-Host {_ps_literal(f"==> Executing: {display_command}")}
& $exe @argsList
exit $LASTEXITCODE
""".strip()

    encoded = base64.b64encode(remote_script.encode("utf-16le")).decode("ascii")
    remote_command = [
        "vagrant",
        "winrm",
        args.vm,
        "--command",
        f"powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -EncodedCommand {encoded}",
    ]

    print(f"==> Executing xp2p on guest {args.vm}")
    returncode, stdout_text, stderr_text = _run_stream(remote_command, cwd=vm_dir)

    if returncode != 0:
        raise RuntimeError(
            f"Remote command exited with code {returncode}."
        )

    print("==> Remote execution completed successfully")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("==> Operation interrupted by user", file=sys.stderr)
        raise SystemExit(130)
    except Exception as exc:
        print(f"Error: {exc}", file=sys.stderr)
        raise SystemExit(1)
