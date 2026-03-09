# xp2p ping

xp2p ping <host>

Options:
- --count, -c <n>          Number of echo requests to send
- --timeout, -t <sec>      Per-request timeout in seconds
- --proto, -o <tcp|udp>    Protocol to use
- --port, -P <n>           Target port (default 62022)
- --tunnel, -T [host:port] Route ping through xp2p tunnel (SOCKS5). If value omitted, auto-detect from config
- --endpoint, -e <tag>     Endpoint tag to use when multiple endpoints share the same host
- --index, -i <n>          Endpoint index (1-based) to use when multiple endpoints share the same host
- Global options
