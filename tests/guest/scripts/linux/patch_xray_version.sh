#!/bin/bash
set -euo pipefail

if [ "$#" -ne 4 ]; then
  echo "Usage: patch_xray_version.sh <xray_path> <backup_path> <expected_version> <replacement_version>" >&2
  exit 2
fi

XRAY_PATH=$1
BACKUP_PATH=$2
EXPECTED_VERSION=$3
REPLACEMENT_VERSION=$4

if [ ! -f "$XRAY_PATH" ]; then
  echo "xray binary not found: $XRAY_PATH" >&2
  exit 3
fi

if [ "${#EXPECTED_VERSION}" -ne "${#REPLACEMENT_VERSION}" ]; then
  echo "Expected and replacement versions must have equal length." >&2
  exit 4
fi

if [ ! -f "$BACKUP_PATH" ]; then
  cp "$XRAY_PATH" "$BACKUP_PATH"
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT
cat >"$tmp_dir/patch_xray.go" <<'EOF'
package main

import (
	"bytes"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: patch_xray <path> <expected> <replacement>")
		os.Exit(2)
	}
	path := os.Args[1]
	expected := os.Args[2]
	replacement := os.Args[3]
	if len(expected) != len(replacement) {
		fmt.Fprintln(os.Stderr, "expected and replacement must be same length")
		os.Exit(2)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read xray: %v\n", err)
		os.Exit(3)
	}
	count := bytes.Count(data, []byte(expected))
	if count == 0 {
		fmt.Fprintf(os.Stderr, "version string %q not found in %s\n", expected, path)
		os.Exit(4)
	}

	data = bytes.ReplaceAll(data, []byte(expected), []byte(replacement))
	info, err := os.Stat(path)
	mode := os.FileMode(0o755)
	if err == nil {
		mode = info.Mode()
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		fmt.Fprintf(os.Stderr, "write xray: %v\n", err)
		os.Exit(5)
	}
	fmt.Printf("__XP2P_REPLACED__=%d\n", count)
}
EOF

go run "$tmp_dir/patch_xray.go" "$XRAY_PATH" "$EXPECTED_VERSION" "$REPLACEMENT_VERSION"
