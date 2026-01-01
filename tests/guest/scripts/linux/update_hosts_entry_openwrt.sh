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

TARGET_HOST=$(printf '%s' "$HOST_NAME" | tr 'A-Z' 'a-z')
TMP_FILE="$(mktemp)"

awk -v host="$TARGET_HOST" -v action="$ACTION" -v ip="$IP_ADDR" -v name="$HOST_NAME" '
function is_comment(line) { return match(line, /^[[:space:]]*#/) }
{
  line = $0
  if (is_comment(line) || line ~ /^[[:space:]]*$/) {
    print line
    next
  }
  n = split(line, fields, /[[:space:]]+/)
  if (n > 1) {
    for (i = 2; i <= n; i++) {
      if (fields[i] ~ /^#/) {
        break
      }
      if (tolower(fields[i]) == host) {
        next
      }
    }
  }
  print line
}
END {
  if (action == "add" && ip != "") {
    print ip " " name
  }
}
' /etc/hosts > "$TMP_FILE"

cat "$TMP_FILE" > /etc/hosts
rm -f "$TMP_FILE"
