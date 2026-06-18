#!/bin/bash
set -euo pipefail

TARGET=${1:-/etc/xp2p/bin/xray}
LOG=${2:-/tmp/xp2p-loop-spike-xray.log}
FD_COUNT=${3:-1200}

mkdir -p "$(dirname "$TARGET")"
TMP_TARGET="$TARGET.loop-spike-tmp"
cat >"$TMP_TARGET" <<'PY'
#!/usr/bin/env python3
import json
import signal
import socket
import sys
import time


def version() -> int:
    print("Xray 0.0.0 (synthetic loop spike)")
    return 0


def config_path(argv: list[str]) -> str:
    for index, arg in enumerate(argv):
        if arg == "-config" and index + 1 < len(argv):
            return argv[index + 1]
    raise SystemExit("missing -config")


def socks_endpoint(path: str) -> tuple[str, int]:
    with open(path, "r", encoding="utf-8") as handle:
        data = json.load(handle)
    for inbound in data.get("inbounds", []) or []:
        if inbound.get("protocol") != "socks":
            continue
        host = str(inbound.get("listen") or "127.0.0.1")
        port = int(inbound.get("port") or 0)
        if port > 0:
            return host, port
    return "127.0.0.1", 1080


def main(argv: list[str]) -> int:
    if "-version" in argv or "version" in argv:
        return version()

    log_path = sys.argv[0] + ".log"
    target_fd_count = int(sys.argv[0].split(".fd")[-1]) if ".fd" in sys.argv[0] else 1200
    config = config_path(argv)
    host, port = socks_endpoint(config)

    sockets: list[socket.socket] = []
    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind((host, port))
    listener.listen(64)
    sockets.append(listener)

    stop = False

    def _stop(_signum, _frame):
        nonlocal stop
        stop = True

    signal.signal(signal.SIGTERM, _stop)
    signal.signal(signal.SIGINT, _stop)

    with open(log_path, "a", encoding="utf-8") as log:
        log.write(f"synthetic xray listening on {host}:{port}\n")
        log.flush()
        time.sleep(1.5)
        for _ in range(target_fd_count):
            try:
                sockets.append(socket.socket(socket.AF_INET, socket.SOCK_STREAM))
            except OSError:
                break
        log.write(f"synthetic xray opened sockets={len(sockets)}\n")
        log.flush()
        while not stop:
            try:
                listener.settimeout(0.5)
                conn, _addr = listener.accept()
                conn.close()
            except socket.timeout:
                pass
            except OSError as exc:
                log.write(f"listener error: {exc}\n")
                log.flush()
                time.sleep(0.5)

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
PY

chmod 0755 "$TMP_TARGET"
rm -f "$TARGET" "$TARGET.fd$FD_COUNT"
mv "$TMP_TARGET" "$TARGET.fd$FD_COUNT"
ln -sf "$(basename "$TARGET").fd$FD_COUNT" "$TARGET"
rm -f "$TARGET.log" "$TARGET.fd$FD_COUNT.log"
touch "$LOG"
ln -sf "$LOG" "$TARGET.log"
ln -sf "$LOG" "$TARGET.fd$FD_COUNT.log"
