#!/bin/sh
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
BUILD_ROOT=${XP2P_BUILD_ROOT:-"/tmp/build"}
TARGET_FILTER=${XP2P_TARGETS:-""}
LDFLAGS=${XP2P_LDFLAGS:-""}
CGO_OPTION=${XP2P_CGO_ENABLED:-0}
STRIP_ENABLE=${XP2P_STRIP:-1}
STRIP_BIN=${XP2P_STRIP_BIN:-strip}
GOEXPERIMENT_OPT=${XP2P_GOEXPERIMENT:-""}

usage() {
  cat <<'EOF'
Usage: scripts/build/build_xp2p_binaries.sh

Builds the xp2p CLI for selected targets and places the binaries under /tmp/build
(override via XP2P_BUILD_ROOT). Targets must be specified via --targets/--target
(comma or space separated GOOS-GOARCH identifiers) or by setting XP2P_TARGETS.
EOF
}

TARGET_ARGS=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --targets)
      shift
      [ "$#" -eq 0 ] && { echo "ERROR: --targets requires an argument" >&2; exit 2; }
      TARGET_ARGS="$TARGET_ARGS $1"
      shift
      ;;
    --target)
      shift
      [ "$#" -eq 0 ] && { echo "ERROR: --target requires an argument" >&2; exit 2; }
      TARGET_ARGS="$TARGET_ARGS $1"
      shift
      ;;
    --)
      shift
      break
      ;;
    -*)
      echo "ERROR: unknown flag $1" >&2
      usage
      exit 2
      ;;
    *)
      echo "ERROR: unexpected argument $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [ -n "$TARGET_ARGS" ]; then
  TARGET_FILTER=$(printf "%s %s" "$TARGET_FILTER" "$TARGET_ARGS")
fi

if [ -z "$(printf "%s" "$TARGET_FILTER" | tr -d '[:space:],')" ]; then
  echo "ERROR: specify targets via --targets/--target or XP2P_TARGETS" >&2
  exit 2
fi

normalize_targets() {
  printf "%s\n" "$TARGET_FILTER" | tr ',\t ' '\n' | sed '/^$/d'
}

TARGETS=$(normalize_targets)

if [ -z "$LDFLAGS" ]; then
  VERSION=$(cd "$PROJECT_ROOT" && go run ./go/cmd/xp2p --version)
  LDFLAGS="-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$VERSION"
fi

export CGO_ENABLED="$CGO_OPTION"
export GOEXPERIMENT="$GOEXPERIMENT_OPT"

mkdir -p "$BUILD_ROOT"

for target in $TARGETS; do
  out_dir="${BUILD_ROOT%/}/$target"
  mkdir -p "$out_dir"
  echo "==> Building xp2p for $target into $out_dir"
  (cd "$PROJECT_ROOT" && \
    go run ./go/tools/targets build \
      --target "$target" \
      --out-dir "$out_dir" \
      --binary xp2p \
      --pkg ./go/cmd/xp2p \
      --ldflags "$LDFLAGS")

  if [ "$STRIP_ENABLE" = "1" ] && command -v "$STRIP_BIN" >/dev/null 2>&1; then
    binary_path="$out_dir/xp2p"
    if [ ! -f "$binary_path" ] && [ -f "${binary_path}.exe" ]; then
      binary_path="${binary_path}.exe"
    fi
    if [ -f "$binary_path" ]; then
      "$STRIP_BIN" --strip-unneeded "$binary_path" >/dev/null 2>&1 || true
    fi
  fi
done

echo "xp2p binaries are available under $BUILD_ROOT"
