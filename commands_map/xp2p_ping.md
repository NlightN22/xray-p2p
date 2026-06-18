# xp2p ping

## Command tree

xp2p ping <host>
  Summary: Send diagnostic ping requests to xp2p agents
Options:
Includes: inherited options
- --continuous, -C send ping requests until interrupted
- --count, -N <n> number of echo requests to send
- --endpoint, -e <id> endpoint tag to use when multiple endpoints share the same host
- --index, -i <n> endpoint index (1-based) to use when multiple endpoints share the same host
- --keep-open, -k keep one TCP connection open and fail when it breaks
- --port, -P <port> target port (default 62022)
- --proto, -o <proto> protocol to use (tcp or udp)
- --timeout, -t <duration> per-request timeout in seconds (optional)
- --tunnel, -T <string> route ping through xp2p tunnel (SOCKS5 host:port); omit value to auto-detect from xp2p config

