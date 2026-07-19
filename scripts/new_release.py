#!/usr/bin/env python3
import argparse
import os
import re
import shutil
import subprocess
import sys
import threading
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


@dataclass(frozen=True)
class Options:
    command: str
    version: str
    dry_run: bool
    log_file: str | None
    keepalive_seconds: int


def _project_root() -> Path:
    return Path(__file__).resolve().parent.parent


def _ts() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


class _Logger:
    def __init__(self, *, log_file: Path | None) -> None:
        self._log_file = log_file
        self._lock = threading.Lock()
        if self._log_file is not None:
            self._log_file.parent.mkdir(parents=True, exist_ok=True)

    def line(self, text: str) -> None:
        message = f"{_ts()} {text}"
        with self._lock:
            print(message, flush=True)
            if self._log_file is not None:
                with self._log_file.open("a", encoding="utf-8", newline="\n") as f:
                    f.write(message + "\n")


def _write_section(log: _Logger, message: str) -> None:
    log.line(f"=== {message} ===")


def _confirm(prompt: str, *, default_yes: bool, quiet: bool) -> bool:
    if quiet:
        return True
    suffix = "[Y/n]" if default_yes else "[y/N]"
    answer = input(f"{prompt} {suffix} ").strip()
    if not answer:
        return default_yes
    return answer.lower() in ("y", "yes")


def _run(
    log: _Logger,
    args: list[str],
    *,
    cwd: Path | None = None,
    check: bool = True,
    capture: bool = False,
    dry_run: bool,
    keepalive_seconds: int = 60,
) -> subprocess.CompletedProcess[str]:
    cwd_text = str(cwd) if cwd else None
    if dry_run:
        where = f" (cwd={cwd_text})" if cwd_text else ""
        log.line(f"[dry-run] {shutil.which(args[0]) or args[0]} {' '.join(args[1:])}{where}")
        return subprocess.CompletedProcess(args=args, returncode=0, stdout="", stderr="")

    where = f" (cwd={cwd_text})" if cwd_text else ""
    log.line(f"[run] {shutil.which(args[0]) or args[0]} {' '.join(args[1:])}{where}")
    started = time.monotonic()

    if capture:
        cp = subprocess.run(
            args,
            cwd=cwd_text,
            check=False,
            text=True,
            capture_output=True,
        )
        duration = time.monotonic() - started
        log.line(f"[exit] code={cp.returncode} duration={duration:.1f}s")
        if check and cp.returncode != 0:
            raise subprocess.CalledProcessError(cp.returncode, cp.args, output=cp.stdout, stderr=cp.stderr)
        return cp

    proc = subprocess.Popen(
        args,
        cwd=cwd_text,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
    )
    assert proc.stdout is not None

    last_output: list[float] = [time.monotonic()]
    stop_event = threading.Event()

    def keepalive() -> None:
        if keepalive_seconds <= 0:
            return
        while not stop_event.wait(timeout=1.0):
            now = time.monotonic()
            if now - last_output[0] >= keepalive_seconds:
                elapsed = now - started
                log.line(f"[still running] elapsed={elapsed:.1f}s")
                last_output[0] = now

    t = threading.Thread(target=keepalive, name="new-release-keepalive", daemon=True)
    t.start()

    try:
        for line in proc.stdout:
            last_output[0] = time.monotonic()
            log.line(line.rstrip("\n"))
    except KeyboardInterrupt:
        log.line("[interrupt] received Ctrl+C, terminating child process...")
        try:
            proc.terminate()
        except OSError:
            pass
        raise
    finally:
        stop_event.set()
        t.join(timeout=2.0)

    rc = proc.wait()
    duration = time.monotonic() - started
    log.line(f"[exit] code={rc} duration={duration:.1f}s")
    if check and rc != 0:
        raise subprocess.CalledProcessError(rc, args)
    return subprocess.CompletedProcess(args=args, returncode=rc, stdout="", stderr="")


def _require_cmd(name: str) -> None:
    if shutil.which(name) is None:
        raise SystemExit(f"{name} is required")


def _read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def _write_text_lf_no_bom(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    normalized = content.replace("\r\n", "\n").replace("\r", "\n")
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_bytes(normalized.encode("utf-8"))
    tmp.replace(path)


def _git_output(log: _Logger, args: list[str], *, dry_run: bool, keepalive_seconds: int) -> str:
    cp = _run(log, ["git", *args], capture=True, check=True, dry_run=dry_run, keepalive_seconds=keepalive_seconds)
    return (cp.stdout or "").strip()


def _git_check(log: _Logger, args: list[str], *, dry_run: bool, keepalive_seconds: int) -> bool:
    if dry_run:
        _run(log, ["git", *args], capture=True, check=False, dry_run=True, keepalive_seconds=keepalive_seconds)
        return False
    cp = _run(log, ["git", *args], capture=True, check=False, dry_run=dry_run, keepalive_seconds=keepalive_seconds)
    return cp.returncode == 0


def _parse_openwrt_arch_from_ipk_name(name: str) -> str:
    m = re.match(r"^.+_\d+\.\d+\.\d+-\d+_(?P<arch>.+)\.ipk$", name)
    if not m:
        return ""
    return (m.group("arch") or "").strip()


def _cleanup_openwrt_artifacts(artifacts_dir: Path) -> None:
    if not artifacts_dir.exists():
        return
    for pattern in ("*.ipk", "Packages", "Packages.gz", "Packages.sig", "index.html"):
        for p in artifacts_dir.glob(pattern):
            try:
                p.unlink()
            except OSError:
                pass

    for p in artifacts_dir.glob("*.tmp"):
        try:
            p.unlink()
        except OSError:
            pass

    try:
        if artifacts_dir.exists() and not any(artifacts_dir.iterdir()):
            shutil.rmtree(artifacts_dir, ignore_errors=True)
    except OSError:
        pass

    staging_root = artifacts_dir.parent.parent if artifacts_dir.parts[-2:] == ("staging", "stable") else None
    if staging_root and staging_root.exists():
        try:
            if not any(staging_root.iterdir()):
                shutil.rmtree(staging_root, ignore_errors=True)
        except OSError:
            pass


def _update_go_version_file(log: _Logger, *, version: str, dry_run: bool, keepalive_seconds: int) -> None:
    version_file = Path("go/internal/version/version.go")
    if not version_file.exists():
        raise SystemExit(f"Version file not found at {version_file}")

    _write_section(log, "Updating version file")
    original = _read_text(version_file)
    pattern = r'var current = ".*"'
    if not re.search(pattern, original):
        raise SystemExit(f"Version placeholder not found in {version_file}")
    updated = re.sub(pattern, f'var current = "{version}"', original, count=1)
    if original == updated:
        _write_section(log, f"Version file already set to {version}")
        return

    if dry_run:
        log.line(f"[dry-run] write {version_file}")
    else:
        version_file.write_text(updated, encoding="utf-8")
        _run(log, ["gofmt", "-w", str(version_file)], dry_run=dry_run, keepalive_seconds=keepalive_seconds)


def _run_make_build_deb(log: _Logger, *, quiet: bool, dry_run: bool, keepalive_seconds: int) -> None:
    _write_section(log, "Running make build-deb")
    if not _confirm("Run make build-deb", default_yes=False, quiet=quiet):
        log.line("Skipped make build-deb")
        return
    _run(log, ["make", "build-deb"], dry_run=dry_run, keepalive_seconds=keepalive_seconds)


def _update_openwrt_package_makefile(log: _Logger, *, version: str, dry_run: bool) -> None:
    makefile = Path("openwrt/feed/packages/utils/xp2p/Makefile")
    if not makefile.exists():
        raise SystemExit(f"OpenWrt package Makefile not found at {makefile}")

    _write_section(log, "Updating OpenWrt package version")
    content = _read_text(makefile)

    pkg_pattern = re.compile(r"(?m)^(PKG_VERSION:=)(.*)$")
    rel_pattern = re.compile(r"(?m)^(PKG_RELEASE:=)(.*)$")

    updated = pkg_pattern.sub(rf"\g<1>{version}", content, count=1)
    updated = rel_pattern.sub(r"\g<1>1", updated, count=1)

    if content == updated:
        _write_section(log, f"OpenWrt package version already set to {version}")
        return

    if dry_run:
        log.line(f"[dry-run] write {makefile}")
        return
        _write_text_lf_no_bom(makefile, updated)


def _assert_release_version(version: str) -> None:
    go_content = _read_text(Path("go/internal/version/version.go"))
    go_match = re.search(r'var current = "([^"]+)"', go_content)
    package_content = _read_text(Path("openwrt/feed/packages/utils/xp2p/Makefile"))
    package_match = re.search(r"(?m)^PKG_VERSION:=(.+)$", package_content)
    go_version = go_match.group(1).strip() if go_match else ""
    package_version = package_match.group(1).strip() if package_match else ""
    if go_version != version or package_version != version:
        raise SystemExit(
            f"release version mismatch: requested {version}, "
            f"Go has {go_version or '(missing)'}, OpenWrt has {package_version or '(missing)'}"
        )


def _build_openwrt_ipk(
    log: _Logger,
    *,
    artifacts_dir: Path,
    expected_arches: list[str],
    quiet: bool,
    dry_run: bool,
    keepalive_seconds: int,
) -> None:
    _write_section(log, f"Building OpenWrt .ipk into {artifacts_dir.as_posix()}")
    if not _confirm(
        f"Build OpenWrt .ipk (Vagrant) and stage into {artifacts_dir.as_posix()}",
        default_yes=True,
        quiet=quiet,
    ):
        log.line("Skipped OpenWrt .ipk build/stage")
        return

    vagrant_dir = Path("infra/vagrant/debian12/ipk-build")
    if not vagrant_dir.exists():
        raise SystemExit(f"Vagrant directory not found at {vagrant_dir}")

    artifacts_dir.mkdir(parents=True, exist_ok=True)
    guest_dir = f"/srv/xray-p2p/{artifacts_dir.as_posix()}"
    inner = f"bash /srv/xray-p2p/scripts/build/build_openwrt_ipk.sh --all --force-build --output-dir {guest_dir}"
    cmd = f'bash -lc "{inner}"'

    _run(log, ["vagrant", "up"], cwd=vagrant_dir, dry_run=dry_run, keepalive_seconds=keepalive_seconds)
    _run(log, ["vagrant", "ssh", "-c", cmd], cwd=vagrant_dir, dry_run=dry_run, keepalive_seconds=keepalive_seconds)

    built = sorted(p for p in artifacts_dir.glob("*.ipk") if p.is_file())
    if not built:
        raise SystemExit(f"No .ipk files found under {artifacts_dir} after build")

    found: set[str] = set()
    for p in built:
        arch = _parse_openwrt_arch_from_ipk_name(p.name)
        if arch:
            found.add(arch)

    missing = sorted(a for a in expected_arches if a not in found)
    if missing:
        have = ", ".join(sorted(found)) if found else "(none)"
        need = ", ".join(sorted(expected_arches))
        miss = ", ".join(missing)
        raise SystemExit(
            "OpenWrt build produced incomplete set of .ipk files. "
            f"Missing arches: {miss}. Have: {have}. Expected: {need}."
        )

    log.line(f"Built {len(built)} .ipk files into {artifacts_dir}")


def _create_release_commit_if_needed(log: _Logger, *, tag: str, dry_run: bool, keepalive_seconds: int) -> None:
    pending = _git_output(log, ["status", "--porcelain", "--untracked-files=no"], dry_run=dry_run, keepalive_seconds=keepalive_seconds)
    if not pending.strip():
        _write_section(log, "No changes to commit; reusing current HEAD for tagging")
        return

    _write_section(log, "Creating release commit")
    _run(log, ["git", "commit", "-am", f"chore: release {tag}"], dry_run=dry_run, keepalive_seconds=keepalive_seconds)


def _create_and_push_tag_and_main(log: _Logger, *, tag: str, quiet: bool, dry_run: bool, keepalive_seconds: int) -> None:
    _write_section(log, f"Tagging {tag}")
    _run(log, ["git", "tag", tag], dry_run=dry_run, keepalive_seconds=keepalive_seconds)

    _write_section(log, "Pushing branch main")
    if _confirm("Push branch main to origin?", default_yes=False, quiet=quiet):
        _run(log, ["git", "push", "origin", "main"], dry_run=dry_run, keepalive_seconds=keepalive_seconds)
    else:
        log.line("Skipping push of branch main")

    _write_section(log, f"Pushing tag {tag}")
    if _confirm(f"Push tag {tag} to origin?", default_yes=False, quiet=quiet):
        _run(log, ["git", "push", "origin", tag], dry_run=dry_run, keepalive_seconds=keepalive_seconds)
    else:
        log.line(f"Skipping push of tag {tag}")


def _publish_openwrt_artifacts(
    log: _Logger,
    *,
    tag: str,
    artifacts_branch: str,
    artifacts_dir: Path,
    quiet: bool,
    dry_run: bool,
    keepalive_seconds: int,
) -> None:
    _write_section(log, f"Publishing OpenWrt .ipk to branch {artifacts_branch}")
    if not _confirm(
        f"Commit and push {artifacts_dir.as_posix()}/*.ipk to branch {artifacts_branch}",
        default_yes=True,
        quiet=quiet,
    ):
        log.line("Skipped artifacts branch publish")
        return

    ipks = sorted(p for p in artifacts_dir.glob("*.ipk") if p.is_file())
    if not ipks:
        raise SystemExit(f"No .ipk files found under {artifacts_dir}")

    has_remote = _git_check(
        log,
        ["ls-remote", "--exit-code", "origin", f"refs/heads/{artifacts_branch}"],
        dry_run=dry_run,
        keepalive_seconds=keepalive_seconds,
    )

    worktree_root = Path(".tmp/artifacts-worktree")
    worktree_root.parent.mkdir(parents=True, exist_ok=True)

    _run(
        log,
        ["git", "worktree", "remove", "--force", str(worktree_root)],
        check=False,
        dry_run=dry_run,
        keepalive_seconds=keepalive_seconds,
    )
    if not dry_run:
        shutil.rmtree(worktree_root, ignore_errors=True)

    if has_remote:
        _run(log, ["git", "fetch", "origin", artifacts_branch], dry_run=dry_run, keepalive_seconds=keepalive_seconds)
        _run(
            log,
            ["git", "worktree", "add", "-B", artifacts_branch, str(worktree_root), f"origin/{artifacts_branch}"],
            dry_run=dry_run,
            keepalive_seconds=keepalive_seconds,
        )
    else:
        _run(
            log,
            ["git", "worktree", "add", "-B", artifacts_branch, str(worktree_root)],
            dry_run=dry_run,
            keepalive_seconds=keepalive_seconds,
        )

    try:
        dest_dir = worktree_root / artifacts_dir.as_posix()
        if dry_run:
            log.line(f"[dry-run] prepare {dest_dir}")
        else:
            dest_dir.mkdir(parents=True, exist_ok=True)
            for old in dest_dir.glob("*.ipk"):
                try:
                    old.unlink()
                except OSError:
                    pass
            for p in ipks:
                shutil.copy2(p, dest_dir / p.name)

        git_path = artifacts_dir.as_posix()
        _run(
            log,
            ["git", "-C", str(worktree_root), "add", "--", git_path],
            dry_run=dry_run,
            keepalive_seconds=keepalive_seconds,
        )

        staged = _git_output(
            log,
            ["-C", str(worktree_root), "diff", "--cached", "--name-only"],
            dry_run=dry_run,
            keepalive_seconds=keepalive_seconds,
        )
        message = f"chore(openwrt): stage ipk for {tag}"
        if staged.strip():
            _run(
                log,
                ["git", "-C", str(worktree_root), "commit", "-m", message],
                dry_run=dry_run,
                keepalive_seconds=keepalive_seconds,
            )
        else:
            log.line(f"No changes to commit on {artifacts_branch}")

        if _confirm(f"Push branch {artifacts_branch} to origin?", default_yes=False, quiet=quiet):
            _run(
                log,
                ["git", "-C", str(worktree_root), "push", "-u", "origin", artifacts_branch],
                dry_run=dry_run,
                keepalive_seconds=keepalive_seconds,
            )
            if not dry_run:
                _cleanup_openwrt_artifacts(artifacts_dir)
        else:
            log.line(f"Skipping push of branch {artifacts_branch}")
    finally:
        _run(
            log,
            ["git", "worktree", "remove", "--force", str(worktree_root)],
            check=False,
            dry_run=dry_run,
            keepalive_seconds=keepalive_seconds,
        )
        if not dry_run:
            shutil.rmtree(worktree_root, ignore_errors=True)


def _check_replace_tag(log: _Logger, *, tag: str, replace: bool, dry_run: bool, keepalive_seconds: int) -> None:
    _write_section(log, f"Checking for existing tag {tag}")
    local_exists = _git_check(log, ["rev-parse", "-q", "--verify", tag], dry_run=dry_run, keepalive_seconds=keepalive_seconds)
    remote_exists = _git_check(
        log,
        ["ls-remote", "--exit-code", "origin", f"refs/tags/{tag}"],
        dry_run=dry_run,
        keepalive_seconds=keepalive_seconds,
    )

    if not replace:
        if local_exists:
            raise SystemExit(f"Tag {tag} already exists locally (use --replace-tag to recreate it)")
        if remote_exists:
            raise SystemExit(f"Tag {tag} already exists on origin (use --replace-tag to recreate it)")
        return

    _write_section(log, f"Replacing existing tag {tag}")
    if local_exists:
        _run(log, ["git", "tag", "-d", tag], dry_run=dry_run, keepalive_seconds=keepalive_seconds)
    if remote_exists:
        _run(log, ["git", "push", "--delete", "origin", tag], dry_run=dry_run, keepalive_seconds=keepalive_seconds)


def run_release(opts: Options) -> int:
    os.chdir(_project_root())
    log_path = Path(opts.log_file) if opts.log_file else None
    log = _Logger(log_file=log_path)

    _require_cmd("git")
    _require_cmd("go")

    if opts.command == "check":
        _write_section(log, "Release preflight")
        status = _git_output(log, ["status", "--short"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        log.line("Working tree changes:\n" + (status or "(none)"))
        for target in ("schema-check", "schema-test", "schema-compat", "test-wsl"):
            _run(log, ["make", target], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        return 0

    version = opts.version.strip()
    if not re.match(r"^\d+\.\d+\.\d+$", version):
        raise SystemExit("Version should be X.Y.Z without leading 'v' (e.g. 0.2.0)")

    tag = f"v{version}"
    if opts.command == "prepare":
        status = _git_output(log, ["status", "--porcelain"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        if status:
            raise SystemExit("prepare requires a clean working tree")
        _check_replace_tag(log, tag=tag, replace=False, dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        _update_go_version_file(log, version=version, dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        _update_openwrt_package_makefile(log, version=version, dry_run=opts.dry_run)
        _run(log, ["make", "schema"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        for target in ("schema-check", "schema-test", "schema-compat", "test", "test-wsl"):
            _run(log, ["make", target], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        _write_section(log, "Prepared release diff")
        _run(log, ["git", "diff", "--", "go/internal/version/version.go", "openwrt/feed/packages/utils/xp2p/Makefile", "schemas"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        return 0

    _check_replace_tag(log, tag=tag, replace=False, dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
    _run(log, ["make", "schema-check"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
    _assert_release_version(version)
    allowed = {"go/internal/version/version.go", "openwrt/feed/packages/utils/xp2p/Makefile", "schemas/xp2p-client.schema.json", "schemas/xp2p-server.schema.json"}
    changed = _git_output(log, ["status", "--porcelain"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
    paths = {line[3:].strip().replace("\\", "/") for line in changed.splitlines() if len(line) > 3}
    unexpected = sorted(paths - allowed)
    if unexpected:
        raise SystemExit("publish found non-release changes: " + ", ".join(unexpected))
    if paths:
        _run(log, ["git", "add", "--", *sorted(paths)], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
        _run(log, ["git", "commit", "-m", f"chore: release {tag}"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
    else:
        _run(log, ["git", "commit", "--allow-empty", "-m", f"chore: release {tag}"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
    _run(log, ["git", "tag", "-a", tag, "-m", f"Release {tag}"], dry_run=opts.dry_run, keepalive_seconds=opts.keepalive_seconds)
    log.line(f"Created local release commit and annotated tag {tag}; review before any push.")
    return 0


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(prog="new_release")
    ap.add_argument("command", choices=("check", "prepare", "publish"))
    ap.add_argument("--version", default="", help="X.Y.Z version required by prepare and publish")
    ap.add_argument("--dry-run", action="store_true", help="print commands without executing them")
    ap.add_argument("--log-file", default=None, help="also write logs to file (UTF-8, LF)")
    ap.add_argument(
        "--keepalive-seconds",
        type=int,
        default=60,
        help="print a keepalive line if a command is silent for N seconds (0 disables)",
    )

    args = ap.parse_args(argv)
    opts = Options(
        command=str(args.command),
        version=args.version,
        dry_run=bool(args.dry_run),
        log_file=str(args.log_file) if args.log_file else None,
        keepalive_seconds=int(args.keepalive_seconds),
    )
    return run_release(opts)


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
