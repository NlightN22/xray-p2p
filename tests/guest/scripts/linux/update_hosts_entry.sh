#!/bin/sh
set -eu

usage() {
  echo "Usage: $0 add <ip> <hostname> | remove <hostname>" >&2
  exit 2
}

if [ "$#" -lt 2 ]; then
  usage
fi

ACTION="$1"

case "$ACTION" in
  add)
    if [ "$#" -ne 3 ]; then
      usage
    fi
    IP_ADDR="$2"
    HOST_NAME="$3"
    ;;
  remove)
    if [ "$#" -ne 2 ]; then
      usage
    fi
    IP_ADDR=""
    HOST_NAME="$2"
    ;;
  *)
    usage
    ;;
esac

if command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
else
  PYTHON_BIN="python"
fi

"$PYTHON_BIN" - "$ACTION" "$HOST_NAME" "$IP_ADDR" <<'PY'
import pathlib
import sys

action = sys.argv[1]
host = sys.argv[2].strip()
ip = sys.argv[3].strip() if action == "add" else ""
path = pathlib.Path("/etc/hosts")
lines = []
if path.exists():
    lines = path.read_text().splitlines()

target = host.lower()

def filter_lines(content: list[str]) -> list[str]:
    result: list[str] = []
    for line in content:
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            result.append(line)
            continue
        fields = stripped.split()
        keep = True
        # Skip the first field (IP)
        for field in fields[1:]:
            if field.startswith("#"):
                break
            if field.lower() == target:
                keep = False
                break
        if keep:
            result.append(line)
    return result

filtered = filter_lines(lines)
if action == "add":
    if not ip:
        raise SystemExit("IP address is required for add action")
    filtered.append(f"{ip} {host}")

if filtered and filtered[-1].strip():
    filtered.append("")

path.write_text("\n".join(filtered))
PY
