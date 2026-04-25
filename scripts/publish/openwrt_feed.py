#!/usr/bin/env python3
import argparse
import hashlib
import re
import shutil
import sys
from dataclasses import dataclass
from pathlib import Path

import yaml


@dataclass(frozen=True, order=True)
class Semver3:
    major: int
    minor: int
    patch: int

    @staticmethod
    def parse(text: str) -> "Semver3":
        text = text.strip()
        parts = text.split(".")
        if len(parts) != 3:
            raise ValueError(f"expected MAJOR.MINOR.PATCH, got {text!r}")
        major, minor, patch = (int(p) for p in parts)
        if major < 0 or minor < 0 or patch < 0:
            raise ValueError(f"negative semver not allowed: {text!r}")
        return Semver3(major=major, minor=minor, patch=patch)


@dataclass(frozen=True)
class ParsedIPK:
    path: Path
    basename: str
    pkg_name: str
    version: Semver3
    pkgrel: int
    arch: str
    sha256: str


@dataclass(frozen=True)
class ParsedName:
    path: Path
    basename: str
    pkg_name: str
    version: Semver3
    pkgrel: int


@dataclass(frozen=True)
class ManifestInput:
    glob: str
    releases: list[str]


@dataclass(frozen=True)
class Manifest:
    releases: list[str]
    arches: list[str]
    inputs: list[ManifestInput]
    filename_regex: str
    keep_last: int
    channel: str = "stable"


def _read_manifest(path: Path) -> Manifest:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError("manifest must be a mapping")
    if raw.get("version") != 1:
        raise ValueError("unsupported manifest version (expected 1)")

    releases = [str(x) for x in raw.get("releases", [])]
    arches = [str(x) for x in raw.get("arches", [])]
    if not releases:
        raise ValueError("releases must be non-empty")
    if not arches:
        raise ValueError("arches must be non-empty")

    pkg = raw.get("package_filename") or {}
    if not isinstance(pkg, dict) or not str(pkg.get("regex", "")).strip():
        raise ValueError("package_filename.regex must be set")
    filename_regex = str(pkg["regex"])

    retention = raw.get("retention") or {}
    keep_last = int(retention.get("keep_last", 0))
    if keep_last <= 0:
        raise ValueError("retention.keep_last must be > 0")

    channel = str(raw.get("channel", "stable"))

    release_set = set(releases)
    inputs: list[ManifestInput] = []
    for idx, item in enumerate(raw.get("inputs", []) or []):
        if not isinstance(item, dict):
            raise ValueError(f"inputs[{idx}] must be a mapping")
        glob = str(item.get("glob", "")).strip()
        rels = [str(x) for x in (item.get("releases") or [])]
        if not glob:
            raise ValueError(f"inputs[{idx}].glob must be non-empty")
        if not rels:
            raise ValueError(f"inputs[{idx}].releases must be non-empty")
        for r in rels:
            if r not in release_set:
                raise ValueError(f"inputs[{idx}] references unknown release {r!r}")
        inputs.append(ManifestInput(glob=glob, releases=rels))
    if not inputs:
        raise ValueError("inputs must be non-empty")

    return Manifest(
        releases=releases,
        arches=arches,
        inputs=inputs,
        filename_regex=filename_regex,
        keep_last=keep_last,
        channel=channel,
    )


def _sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def _parse_ipk_fields(basename: str, name_re: re.Pattern[str]) -> tuple[str, Semver3, int, str]:
    m = name_re.match(basename)
    if not m:
        raise ValueError(f"invalid ipk name {basename!r} (expected {name_re.pattern})")
    pkg_name = (m.group("name") or "").strip()
    version_text = (m.group("version") or "").strip()
    pkgrel_text = (m.group("pkgrel") or "").strip()
    arch = (m.group("arch") or "").strip()
    if not pkg_name or not version_text or not pkgrel_text or not arch:
        raise ValueError(f"invalid ipk name {basename!r}: missing fields")
    version = Semver3.parse(version_text)
    pkgrel = int(pkgrel_text)
    if pkgrel < 0:
        raise ValueError(f"invalid pkgrel in {basename!r}: {pkgrel_text!r}")
    return pkg_name, version, pkgrel, arch


def _parse_ipk(path: Path, name_re: re.Pattern[str]) -> ParsedIPK:
    pkg_name, version, pkgrel, arch = _parse_ipk_fields(path.name, name_re)
    return ParsedIPK(
        path=path,
        basename=path.name,
        pkg_name=pkg_name,
        version=version,
        pkgrel=pkgrel,
        arch=arch,
        sha256=_sha256_file(path),
    )


def _copy_file_checked(src: Path, dst: Path) -> None:
    dst.parent.mkdir(parents=True, exist_ok=True)
    if dst.exists():
        if _sha256_file(dst) != _sha256_file(src):
            raise ValueError(f"conflicting content for {dst.name!r} in {dst.parent}")
        return
    shutil.copy2(src, dst)


def _apply_retention(dir_path: Path, name_re: re.Pattern[str], keep_last: int) -> int:
    grouped: dict[str, list[ParsedName]] = {}
    removed = 0

    for entry in dir_path.iterdir():
        if not entry.is_file() or entry.suffix != ".ipk":
            continue
        pkg_name, version, pkgrel, _arch = _parse_ipk_fields(entry.name, name_re)
        grouped.setdefault(pkg_name, []).append(
            ParsedName(
                path=entry,
                basename=entry.name,
                pkg_name=pkg_name,
                version=version,
                pkgrel=pkgrel,
            )
        )

    for pkg_name, files in grouped.items():
        files.sort(key=lambda f: (f.version, f.pkgrel, f.basename))
        if len(files) <= keep_last:
            continue
        for old in files[: len(files) - keep_last]:
            old.path.unlink()
            removed += 1

    return removed


def _write_lines(path: Path, lines: list[str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text("".join(f"{line}\n" for line in lines), encoding="utf-8")
    tmp.replace(path)


def assemble_to_pages(*, manifest_path: Path, inputs_root: Path, pages_root: Path, state_dir: Path) -> None:
    m = _read_manifest(manifest_path)
    name_re = re.compile(m.filename_regex)

    allowed_arches = set(m.arches)
    release_set = set(m.releases)

    pages_root.mkdir(parents=True, exist_ok=True)
    state_dir.mkdir(parents=True, exist_ok=True)

    for rel in m.releases:
        for arch in m.arches:
            (pages_root / rel / arch).mkdir(parents=True, exist_ok=True)

    parsed_by_basename: dict[str, ParsedIPK] = {}
    input_paths: list[Path] = []

    for in_set in m.inputs:
        for rel in in_set.releases:
            if rel not in release_set:
                raise ValueError(f"unknown release {rel!r} referenced by inputs")

        matches = sorted(inputs_root.glob(in_set.glob))
        if not matches:
            raise ValueError(f"no .ipk files matched {in_set.glob!r} under {inputs_root}")

        for p in matches:
            if not p.is_file():
                continue
            parsed = _parse_ipk(p, name_re)
            if parsed.arch not in allowed_arches:
                raise ValueError(f"unsupported arch {parsed.arch!r} in {parsed.basename}")

            prev = parsed_by_basename.get(parsed.basename)
            if prev is not None and prev.sha256 != parsed.sha256:
                raise ValueError(f"conflicting content for {parsed.basename!r}: {prev.sha256} != {parsed.sha256}")
            parsed_by_basename[parsed.basename] = parsed
            input_paths.append(p)

            for rel in in_set.releases:
                dst_dir = pages_root / rel / parsed.arch
                dst = dst_dir / parsed.basename
                _copy_file_checked(p, dst)

    unique_inputs = sorted({str(p.resolve()) for p in input_paths})
    _write_lines(state_dir / "source-ipks.txt", unique_inputs)

    removed_total = 0
    for rel in m.releases:
        for arch in m.arches:
            removed_total += _apply_retention(pages_root / rel / arch, name_re, m.keep_last)

    _write_lines(state_dir / "retention-removed.txt", [str(removed_total)])


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(prog="openwrt_feed")
    sub = ap.add_subparsers(dest="cmd", required=True)

    assemble = sub.add_parser("assemble", help="assemble/update Pages feed from manifest")
    assemble.add_argument("--manifest", default="openwrt/feed-manifest.yml", help="path to feed manifest")
    assemble.add_argument("--inputs-root", default=".", help="root directory for resolving manifest inputs globs")
    assemble.add_argument("--pages-root", default=".pages", help="directory with Pages content to update")
    assemble.add_argument("--state-dir", default=".tmp/openwrt-feed", help="directory for generated state files")

    args = ap.parse_args(argv)

    if args.cmd == "assemble":
        assemble_to_pages(
            manifest_path=Path(args.manifest),
            inputs_root=Path(args.inputs_root),
            pages_root=Path(args.pages_root),
            state_dir=Path(args.state_dir),
        )
        return 0

    raise AssertionError("unreachable")


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
