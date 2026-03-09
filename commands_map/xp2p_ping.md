# xp2p ping

xp2p ping <host>

Options:
- --count <n>          Number of echo requests to send (no short)
- --timeout <sec>      Per-request timeout in seconds (no short)
- --proto <tcp|udp>    Protocol to use (no short)
- --port <n>           Target port (default 62022) (no short)
- --tunnel [host:port] Route ping through xp2p tunnel (SOCKS5). If value omitted, auto-detect from config (no short)
- --endpoint <tag>     Endpoint tag to use when multiple endpoints share the same host (no short)
- --index <n>          Endpoint index (1-based) to use when multiple endpoints share the same host (no short)
- Global options
