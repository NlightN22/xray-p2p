#!/bin/sh
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
PIN_FILE="$PROJECT_ROOT/go/internal/xray/pinned.json"

if [ ! -f "$PIN_FILE" ]; then
  echo "ERROR: pinned file not found at $PIN_FILE" >&2
  exit 1
fi

export PIN_FILE
export PROJECT_ROOT

python3 - <<'PY'
import hashlib
import json
import os
import sys

project_root = os.environ.get("PROJECT_ROOT")
if not project_root:
    print("ERROR: PROJECT_ROOT is not set", file=sys.stderr)
    sys.exit(1)
pin_file = os.environ.get("PIN_FILE") or os.path.join(project_root, "go", "internal", "xray", "pinned.json")

with open(pin_file, "r", encoding="utf-8") as handle:
    data = json.load(handle)

targets = data.get("targets", {})
errors = []

def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

for target, meta in sorted(targets.items()):
    files = meta.get("files", [])
    if not isinstance(files, list):
        errors.append(f"{target}: files must be a list")
        continue
    if "/" not in target:
        errors.append(f"{target}: expected target format os/arch")
        continue
    os_name, arch = target.split("/", 1)
    for item in files:
        name = item.get("name")
        expected = item.get("sha256")
        required = bool(item.get("required", False))
        if not name or not expected:
            errors.append(f"{target}: invalid file entry {item!r}")
            continue
        path = os.path.join(project_root, "distro", os_name, "bundle", arch, name)
        if not os.path.exists(path):
            if required:
                errors.append(f"{target}: missing {path}")
            continue
        actual = sha256_file(path)
        if actual.lower() != expected.lower():
            errors.append(f"{target}: sha256 mismatch for {path}: expected {expected}, got {actual}")

if errors:
    for err in errors:
        print(f"ERROR: {err}", file=sys.stderr)
    sys.exit(1)

print("Pinned xray assets OK")
PY
