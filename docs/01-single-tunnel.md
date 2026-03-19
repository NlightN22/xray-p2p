# Single Tunnel (A-B)

This section covers the simplest A-B tunnel. Start with deploy, then use install
if you want explicit control.

## Deploy (fastest path)

On B (client), generate the deploy link:

```sh
xp2p client deploy --host 10.63.30.11 --port 62125 --user user@example.com --password secret --trojan-port 58601
```

Copy the link printed in `client deploy: link generated`.

On A (server), run deploy with the link:

```sh
xp2p server deploy --listen :62125 --link "<PASTE_LINK>"
```

## Install (manual path)

On A (server):

```sh
xp2p server install --path /etc/xp2p --config-dir config-server --host 10.63.30.11 --force
```

On B (client), use the link from server install output:

```sh
xp2p client install --path /etc/xp2p --config-dir config-client --link "<PASTE_LINK>" --force
```

## Verify

From B:

```sh
xp2p ping 10.63.30.11 --count 1
```
