#!/usr/bin/env python3
from __future__ import annotations

import argparse
import base64
import re
import subprocess
import sys
import threading
import time
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


def _windows_quote(arg: str) -> str:
    if not arg:
        return '""'
    needs_quotes = any(ch in arg for ch in ' \t"')
    if not needs_quotes:
        return arg
    result = '"'
    backslashes = 0
    for ch in arg:
        if ch == "\\":
            backslashes += 1
        elif ch == '"':
            result += "\\" * (backslashes * 2 + 1)
            result += '"'
            backslashes = 0
        else:
            if backslashes:
                result += "\\" * backslashes
                backslashes = 0
            result += ch
    result += "\\" * (backslashes * 2)
    result += '"'
    return result


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


def _run_stream(
    command: list[str],
    *,
    cwd: Path | None = None,
    poll_callback: callable | None = None,
    poll_interval: float = 0.1,
) -> tuple[int, str, str]:
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

    returncode: int | None = None
    try:
        while True:
            returncode = process.poll()
            if returncode is not None:
                break
            if poll_callback:
                poll_callback()
            time.sleep(poll_interval)
    finally:
        for thread in threads:
            thread.join()

    if poll_callback:
        poll_callback()

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

    log_dir_host = exe_path.parent
    log_dir_host.mkdir(parents=True, exist_ok=True)
    log_filename = f"xp2p-{args.vm}-{int(time.time())}.log"
    log_path_host = log_dir_host / log_filename
    if log_path_host.exists():
        log_path_host.unlink()
    log_path_remote = r"C:\xp2p\build\windows-amd64" + "\\" + log_filename
    print(f"==> Streaming log: {log_path_host}")

    vm_dir = _find_vm_directory(repo_root, args.vm)
    print(f"==> Using Vagrant environment: {vm_dir}")

    print(f"==> Running `vagrant up {args.vm} --no-provision`")
    _run(["vagrant", "up", args.vm, "--no-provision"], cwd=vm_dir)

    display_command = _format_display(xp2p_arguments)
    print(f"==> Effective command: {display_command}")

    remote_exe = r"C:\xp2p\build\windows-amd64\xp2p.exe"
    display_line_literal = _ps_literal(f"==> Executing: {display_command}")
    argument_cli = " ".join(_windows_quote(arg) for arg in xp2p_arguments)
    argument_cli_literal = _ps_literal(argument_cli)
    remote_script = f"""
$ErrorActionPreference = 'Stop'
$exe = {_ps_literal(remote_exe)}
$log = {_ps_literal(log_path_remote)}
if (-not (Test-Path $exe)) {{
    throw "xp2p executable not found at $exe"
}}
$info = Get-Item $exe
$logDir = Split-Path -Parent $log
if ($logDir -and -not (Test-Path $logDir)) {{
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}}
$encoding = New-Object System.Text.UTF8Encoding($false)
$writer = New-Object System.IO.StreamWriter($log, $false, $encoding)
$writer.AutoFlush = $true
$writer.WriteLine("==> Guest binary: " + $info.FullName + " (LastWriteTime: " + $info.LastWriteTime.ToString("u") + ")")
$writer.WriteLine({display_line_literal})
$writer.WriteLine("")
$writer.Dispose()
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $exe
$psi.Arguments = {argument_cli_literal}
$psi.WorkingDirectory = $info.DirectoryName
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.CreateNoWindow = $true
$process = New-Object System.Diagnostics.Process
$process.StartInfo = $psi
if (-not $process.Start()) {{
    throw "Failed to start xp2p process."
}}
$writer = New-Object System.IO.StreamWriter($log, $true, $encoding)
$writer.AutoFlush = $true
$exitCode = -1
try {{
    while (-not $process.HasExited) {{
        while (-not $process.StandardOutput.EndOfStream) {{
            $line = $process.StandardOutput.ReadLine()
            if ($line -ne $null) {{ $writer.WriteLine($line) }}
        }}
        while (-not $process.StandardError.EndOfStream) {{
            $line = $process.StandardError.ReadLine()
            if ($line -ne $null) {{ $writer.WriteLine($line) }}
        }}
        Start-Sleep -Milliseconds 100
    }}
    while (-not $process.StandardOutput.EndOfStream) {{
        $line = $process.StandardOutput.ReadLine()
        if ($line -ne $null) {{ $writer.WriteLine($line) }}
    }}
    while (-not $process.StandardError.EndOfStream) {{
        $line = $process.StandardError.ReadLine()
        if ($line -ne $null) {{ $writer.WriteLine($line) }}
    }}
    $exitCode = $process.ExitCode
}} finally {{
    if ($writer) {{ $writer.Dispose() }}
    $process.Dispose()
}}
Add-Content -Path $log -Value ("==> Exit code: " + $exitCode) -Encoding UTF8
exit $exitCode
""".strip()

    encoded = base64.b64encode(remote_script.encode("utf-16le")).decode("ascii")
    remote_command = [
        "vagrant",
        "winrm",
        args.vm,
        "--command",
        f"powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -EncodedCommand {encoded}",
    ]

    last_log_position = 0

    def drain_log() -> None:
        nonlocal last_log_position
        if not log_path_host.exists():
            return
        with log_path_host.open("r", encoding="utf-8-sig") as log_file:
            log_file.seek(last_log_position)
            data = log_file.read()
            if data:
                print(data, end="")
                last_log_position = log_file.tell()

    print(f"==> Executing xp2p on guest {args.vm}")
    returncode, stdout_text, stderr_text = _run_stream(
        remote_command,
        cwd=vm_dir,
        poll_callback=drain_log,
    )
    drain_log()

    if returncode != 0:
        error_lines = [f"Remote command exited with code {returncode}. See log: {log_path_host}"]
        if stdout_text.strip():
            error_lines.append("WinRM stdout:\n" + stdout_text.strip())
        if stderr_text.strip():
            error_lines.append("WinRM stderr:\n" + stderr_text.strip())
        raise RuntimeError("\n".join(error_lines))

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
   
