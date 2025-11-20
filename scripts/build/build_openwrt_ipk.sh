#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
DEFAULT_BUILD_ROOT="/tmp/build"
CALLER_PWD=$(pwd)

TARGET=""
BUILD_ALL=0
SDK_DIR=""
DIFFCONFIG=""
DIFFCONFIG_OUT=""
BUILD_ROOT=${XP2P_BUILD_ROOT:-$DEFAULT_BUILD_ROOT}
SOURCE_HASH_FILE="$BUILD_ROOT/.xp2p_source_hash"
IPK_CACHE_DIR="$BUILD_ROOT/.xp2p_ipk_cache"
FEED_PATH="$PROJECT_ROOT/openwrt/feed"
FEED_PACKAGE_PATH="$FEED_PATH/packages/utils/xp2p"
REPO_ROOT="$PROJECT_ROOT/openwrt/repo"
RELEASE_VERSION=${OPENWRT_VERSION:-""}
GOTOOLCHAIN_VERSION=${GOTOOLCHAIN:-go1.21.7}
FORCE_BUILD=0
SOURCE_SIGNATURE=""
SOURCE_TRACK_PATHS=(go cmd internal pkg go.mod go.sum)
OUTPUT_DIR=""

usage() {
  cat <<'USAGE'
Usage: build_openwrt_ipk.sh [--target <identifier> | --all] [options]

Options:
  --target <identifier>    Target identifier (e.g. linux-amd64)
  --all                    Build every supported target
  --sdk-dir <path>         Use an existing OpenWrt SDK directory
  --diffconfig <path>      diffconfig applied before make defconfig
  --diffconfig-out <path>  write fresh diffconfig after defconfig
  --build-root <path>      location of prebuilt xp2p/xray/completions (default: /tmp/build)
  --output-dir <path>      store the resulting .ipk/Packages under <path> instead of openwrt/repo/<release>/<arch>
  --force-build            always rebuild xp2p binaries even if sources are unchanged
  -h, --help               Show this message
USAGE
}

while [ "${1:-}" != "" ]; do
  case "$1" in
    --target)
      TARGET="$2"
      shift 2
      ;;
    --all)
      BUILD_ALL=1
      shift
      ;;
    --sdk-dir)
      SDK_DIR="$2"
      shift 2
      ;;
    --diffconfig)
      DIFFCONFIG="$2"
      shift 2
      ;;
    --diffconfig-out)
      DIFFCONFIG_OUT="$2"
      shift 2
      ;;
    --build-root)
      BUILD_ROOT="$2"
      SOURCE_HASH_FILE="$BUILD_ROOT/.xp2p_source_hash"
      IPK_CACHE_DIR="$BUILD_ROOT/.xp2p_ipk_cache"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      case "$OUTPUT_DIR" in
        /*) ;;
        *) OUTPUT_DIR="$CALLER_PWD/$OUTPUT_DIR" ;;
      esac
      shift 2
      ;;
    --force-build)
      FORCE_BUILD=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

SUPPORTED_TARGETS=(linux-386 linux-amd64 linux-armhf linux-arm64 linux-mipsle-softfloat linux-mips64le)

hash_data() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
  else
    python3 - <<'EOF'
import hashlib, sys
print(hashlib.sha256(sys.stdin.buffer.read()).hexdigest())
EOF
  fi
}

file_checksum() {
  local file=$1
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    python3 - "$file" <<'EOF'
import hashlib, sys, pathlib
path = pathlib.Path(sys.argv[1])
with path.open('rb') as handle:
    print(hashlib.sha256(handle.read()).hexdigest())
EOF
  fi
}

compute_source_signature() {
  local available_paths=()
  local rel
  for rel in "${SOURCE_TRACK_PATHS[@]}"; do
    if [ -e "$PROJECT_ROOT/$rel" ]; then
      available_paths+=("$rel")
    fi
  done
  if [ ${#available_paths[@]} -eq 0 ]; then
    echo ""
    return 0
  fi

  local files=()
  for rel in "${available_paths[@]}"; do
    local abs="$PROJECT_ROOT/$rel"
    if [ -d "$abs" ]; then
      while IFS= read -r -d '' file; do
        files+=("$file")
      done < <(find "$abs" -type f -name '*.go' -print0)
    elif [ -f "$abs" ]; then
      files+=("$abs")
    fi
  done
  if [ ${#files[@]} -eq 0 ]; then
    echo ""
    return 0
  fi
  printf "%s\0" "${files[@]}" | LC_ALL=C sort -z | xargs -0 cat | hash_data
}

record_source_signature() {
  local signature=$1
  if [ -z "$signature" ]; then
    return
  fi
  mkdir -p "$(dirname "$SOURCE_HASH_FILE")"
  echo "$signature" > "$SOURCE_HASH_FILE"
}

record_ipk_metadata() {
  local target=$1
  local signature=$2
  local ipk_path=$3
  if [ -z "$signature" ] || [ -z "$ipk_path" ]; then
    return
  fi
  mkdir -p "$IPK_CACHE_DIR"
  {
    printf "signature=%s\n" "$signature"
    printf "ipk_path=%s\n" "$ipk_path"
  } > "$IPK_CACHE_DIR/$target"
}

load_cached_ipk() {
  local target=$1
  local metadata="$IPK_CACHE_DIR/$target"
  if [ ! -f "$metadata" ]; then
    return 1
  fi
  local cached_signature=""
  local cached_ipk=""
  while IFS='=' read -r key value; do
    value=${value%$'\r'}
    case "$key" in
      signature) cached_signature=$value ;;
      ipk_path) cached_ipk=$value ;;
    esac
  done < "$metadata"
  if [ "$cached_signature" != "$SOURCE_SIGNATURE" ]; then
    return 1
  fi
  if [ -z "$cached_ipk" ] || [ ! -f "$cached_ipk" ]; then
    return 1
  fi
  printf "%s" "$cached_ipk"
  return 0
}

ensure_binaries() {
  local target=$1
  local xp2p_bin=$2
  local xray_bin=$3
  local completions_dir=$4
  local needs_build=0
  if [ $FORCE_BUILD -eq 1 ]; then
    needs_build=1
  elif [ ! -x "$xp2p_bin" ] || [ ! -x "$xray_bin" ] || [ ! -d "$completions_dir" ]; then
    needs_build=1
  elif [ -z "$SOURCE_SIGNATURE" ]; then
    needs_build=1
  else
    local cached=""
    if [ -f "$SOURCE_HASH_FILE" ]; then
      cached=$(cat "$SOURCE_HASH_FILE" 2>/dev/null || true)
    fi
    if [ "$cached" != "$SOURCE_SIGNATURE" ]; then
      needs_build=1
    fi
  fi

  if [ $needs_build -eq 1 ]; then
    echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Building xp2p binaries"
    GOTOOLCHAIN=$GOTOOLCHAIN_VERSION "$PROJECT_ROOT/scripts/build/build_xp2p_binaries.sh" --target "$target"
    if [ -n "$SOURCE_SIGNATURE" ]; then
      record_source_signature "$SOURCE_SIGNATURE"
    else
      rm -f "$SOURCE_HASH_FILE" 2>/dev/null || true
    fi
  else
    echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Sources unchanged; reusing cached xp2p binaries"
  fi
}

if [ $BUILD_ALL -eq 1 ]; then
  TARGETS=("${SUPPORTED_TARGETS[@]}")
else
  if [ -z "$TARGET" ]; then
    echo "ERROR: --target is required (or use --all)" >&2
    usage
    exit 1
  fi
  TARGETS=("$TARGET")
fi

if [ $BUILD_ALL -eq 1 ] && [ -n "$SDK_DIR" ]; then
  echo "ERROR: --sdk-dir cannot be combined with --all" >&2
  exit 1
fi

if [ $FORCE_BUILD -eq 0 ]; then
  SOURCE_SIGNATURE=$(compute_source_signature || true)
fi
run_for_target() {
  local target=$1
  local sdk_dir_override=$2

  if [ $FORCE_BUILD -eq 0 ] && [ -n "$SOURCE_SIGNATURE" ]; then
    local cached_ipk_path=""
    cached_ipk_path=$(load_cached_ipk "$target" || true)
    if [ -n "$cached_ipk_path" ]; then
      echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Sources unchanged and cached ipk present at $cached_ipk_path; skipping build"
      return
    fi
  fi

  local sdk_dir="$sdk_dir_override"
  if [ -z "$sdk_dir" ]; then
    sdk_dir="$HOME/openwrt-sdk-$target"
  fi

  local output_dir="${BUILD_ROOT%/}/$target"
  local xp2p_bin="$output_dir/xp2p"
  local xray_bin="$output_dir/xray"
  local completions_dir="$output_dir/completions"

  echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Ensuring OpenWrt SDK"
  "$PROJECT_ROOT/scripts/build/ensure_openwrt_sdk.sh" "$target"

  local release_version="$RELEASE_VERSION"
  if [ -z "$release_version" ]; then
    if [ -f "$sdk_dir/.xp2p-openwrt-version" ]; then
      release_version=$(cut -d'-' -f1 "$sdk_dir/.xp2p-openwrt-version")
    else
      release_version="unknown"
    fi
  fi

  ensure_binaries "$target" "$xp2p_bin" "$xray_bin" "$completions_dir"

  if [ ! -x "$xp2p_bin" ]; then
    echo "ERROR: xp2p binary not found at $xp2p_bin" >&2
    exit 1
  fi
  if [ ! -x "$xray_bin" ]; then
    echo "ERROR: bundled xray not found at $xray_bin" >&2
    exit 1
  fi
  if [ ! -d "$completions_dir" ]; then
    echo "ERROR: completion directory not found at $completions_dir" >&2
    exit 1
  fi

  mkdir -p "$sdk_dir"
  pushd "$sdk_dir" >/dev/null

  if ! grep -qE '^\s*src-link\s+xp2p\s+' feeds.conf.default 2>/dev/null; then
    echo "src-link xp2p $FEED_PATH" >> feeds.conf.default
  fi

  rm -rf feeds/xp2p package/feeds/xp2p 2>/dev/null || true

  echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Updating feed"
  ./scripts/feeds update xp2p
  ./scripts/feeds install xp2p

  local sdk_makefile="package/feeds/xp2p/xp2p/Makefile"
  local repo_makefile="$FEED_PACKAGE_PATH/Makefile"
  if [ -f "$sdk_makefile" ] && ! cmp -s "$sdk_makefile" "$repo_makefile"; then
    echo "WARNING: SDK Makefile differs from repository copy ($sdk_makefile)" >&2
  fi

  if [ -n "$DIFFCONFIG" ]; then
    if [ -f "$DIFFCONFIG" ]; then
      echo "==> [$target] Applying diffconfig from $DIFFCONFIG"
      cp "$DIFFCONFIG" .config
    else
      echo "WARNING: diffconfig $DIFFCONFIG not found, skipping" >&2
    fi
  fi

  echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Running defconfig"
  make defconfig

  if ! grep -q '^CONFIG_PACKAGE_xp2p=y' .config; then
    echo "CONFIG_PACKAGE_xp2p=y" >> .config
    make defconfig
  fi

  if [ -n "$DIFFCONFIG_OUT" ]; then
    echo "==> [$target] Writing diffconfig to $DIFFCONFIG_OUT"
    ./scripts/diffconfig.sh > "$DIFFCONFIG_OUT"
  fi

  echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Building xp2p ipk"
  XP2P_PREBUILT="$xp2p_bin" \
  XP2P_XRAY="$xray_bin" \
  XP2P_COMPLETIONS="$completions_dir" \
    make package/xp2p/clean V=sc >/dev/null 2>&1 || true
  XP2P_PREBUILT="$xp2p_bin" \
  XP2P_XRAY="$xray_bin" \
  XP2P_COMPLETIONS="$completions_dir" \
    make package/xp2p/compile V=sc

  echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Collecting artefact"
  IPK_PATH=$(find "$sdk_dir/bin/packages" -type f -name "xp2p_*.ipk" -print | sort | tail -n1 || true)
  if [ -z "$IPK_PATH" ]; then
    echo "ERROR: xp2p ipk not found in $sdk_dir/bin/packages" >&2
    popd >/dev/null
    exit 1
  fi

  ARCH_DIR=$(tar -xzOf "$IPK_PATH" ./control.tar.gz | tar -xzOf - ./control | awk -F': ' '/^Architecture:/ {print $2; exit}')
  if [ -z "$ARCH_DIR" ]; then
    echo "ERROR: unable to determine architecture for $IPK_PATH" >&2
    popd >/dev/null
    exit 1
  fi

  local dest_dir
  if [ -n "$OUTPUT_DIR" ]; then
    dest_dir="${OUTPUT_DIR%/}"
  else
    dest_dir="$REPO_ROOT/$release_version/$ARCH_DIR"
  fi
  DEST_DIR="$dest_dir"
  mkdir -p "$DEST_DIR"
  cp "$IPK_PATH" "$DEST_DIR/"

  echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') ==> [$target] Updating feed index at $DEST_DIR"
  "$PROJECT_ROOT/scripts/build/make_openwrt_packages.sh" --path "$DEST_DIR"

  if [ -n "$SOURCE_SIGNATURE" ]; then
    record_ipk_metadata "$target" "$SOURCE_SIGNATURE" "$DEST_DIR/$(basename "$IPK_PATH")"
  fi

  popd >/dev/null

  echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') Build complete: $(basename "$IPK_PATH")"
  echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') Stored under: $DEST_DIR"
}

for tgt in "${TARGETS[@]}"; do
  run_for_target "$tgt" "$SDK_DIR"
done
