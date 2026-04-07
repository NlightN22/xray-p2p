#!/bin/sh
set -eu

if [ "$#" -ne 3 ]; then
  echo "Usage: wrap_xray_version.sh <xray_path> <backup_path> <fake_version>" >&2
  exit 2
fi

XRAY_PATH=$1
BACKUP_PATH=$2
FAKE_VERSION=$3

if [ ! -f "$XRAY_PATH" ]; then
  echo "xray binary not found: $XRAY_PATH" >&2
  exit 3
fi

if [ ! -f "$BACKUP_PATH" ]; then
  mv "$XRAY_PATH" "$BACKUP_PATH"
fi

cp "$BACKUP_PATH" "$XRAY_PATH"
cat >"$XRAY_PATH" <<EOF
#!/bin/sh
if [ "\${1:-}" = "-version" ] || [ "\${1:-}" = "--version" ]; then
  echo "Xray ${FAKE_VERSION} (xp2p test wrapper)"
  exit 0
fi
exec "${BACKUP_PATH}" "\$@"
EOF
