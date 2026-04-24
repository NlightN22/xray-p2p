# Single Tunnel (A-B)

This section covers the simplest A-B tunnel. Start with deploy, then use install
if you want explicit control.

## Deploy (fastest path)

On B (client), generate the deploy link:

```sh
xp2p client deploy --host 10.63.30.11
```

Copy the link printed in `client deploy: link generated`.

To keep the client in proxy mode after deploy:

```sh
xp2p client deploy --host 10.63.30.11 --mode proxy
```

To deploy directly into TUN mode (split/full):

```sh
xp2p client deploy --host 10.63.30.11 --mode tun --tun-mode split
```

On A (server), run deploy with the link:

```sh
xp2p server deploy --link "<PASTE_LINK>"
```

If you need a custom deploy port, pass it on both sides:

```sh
xp2p client deploy --host 10.63.30.11 --port 62125
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

## Apply (start services)

Both deploy and install update Desired inputs and write `apply.request`. Start (or restart) the services to apply the changes:

```sh
xp2p server service start
xp2p client service start
```

## Verify

From B:

```sh
xp2p ping 10.63.30.11 --tunnel --count 1
```
