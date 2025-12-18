#!/usr/bin/env bash
# Simple generator of OpenWrt Packages index for a directory with .ipk files

set -euo pipefail

PKG_DIR="."
OUT_FILE="Packages"

# Parse arguments
while [ "$#" -gt 0 ]; do
  case "$1" in
    --path)
      PKG_DIR="$2"
      shift 2
      ;;
    --output)
      OUT_FILE="$2"
      shift 2
      ;;
    --help|-h)
      echo "Usage: $0 [--path DIR] [--output FILE]"
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: $0 [--path DIR] [--output FILE]" >&2
      exit 1
      ;;
  esac
done

cd "$PKG_DIR"

# Truncate output file
: > "$OUT_FILE"

for pkg in *.ipk; do
  # Skip if no .ipk in dir
  [ -e "$pkg" ] || continue

  echo "Indexing $pkg" >&2

  # Get file size and sha256
  size=$(stat -c%s "$pkg")
  sha256=$(sha256sum "$pkg" | awk '{print $1}')

  # Extract control file from .ipk (gzip'ed tar with control.tar.gz inside)
  tar -xzOf "$pkg" ./control.tar.gz | tar -xzOf - ./control | awk -v f="$pkg" -v s="$size" -v h="$sha256" '
    BEGIN { injected = 0 }
    /^Description:/ && !injected {
      print "Filename: " f
      print "Size: " s
      print "SHA256sum: " h
      print $0
      injected = 1
      next
    }
    { print }
  ' >> "$OUT_FILE"

  # Empty line between package entries
  echo >> "$OUT_FILE"
done

if [ ! -s "$OUT_FILE" ]; then
  echo "No .ipk files found in $PKG_DIR" >&2
else
  gzip -9nc "$OUT_FILE" > "${OUT_FILE}.gz"
fi