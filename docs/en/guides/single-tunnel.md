# Single Tunnel (A-B)

This section covers the simplest A-B tunnel. Start with deploy, then use install
if you want explicit control.

## Deploy (fastest path)

On B (client), generate the deploy link:

```sh
xp2p client deploy --host 10.63.30.11
```

Copy the link printed in `client deploy: link generated`.

On A (server), run deploy with the link:

```sh
xp2p server deploy --link "<PASTE_LINK>"
```

By default, deploy enables TUN mode with split routing on the client. Use `--mode proxy` if you want to keep the client in proxy mode.

### Advanced deploy options

Keep the client in proxy mode after deploy:

```sh
xp2p client deploy --host 10.63.30.11 --mode proxy
```

Deploy into full-tunnel TUN mode:

```sh
xp2p client deploy --host 10.63.30.11 --mode tun:full
```

Custom deploy port (pass it on both sides):

```sh
xp2p client deploy --host 10.63.30.11 --port 62125
xp2p server deploy --listen :62125 --link "<PASTE_LINK>"
```

## Install (manual path)

On A (server):

```sh
xp2p server install --host 10.63.30.11
```

On B (client), use the link from server install output:

```sh
xp2p client install --link "<LINK_FROM_SERVER_INSTALL>"
```

## Apply (start services)

Both deploy and install update Desired inputs and write `apply.request`. Start (or restart) the services to apply the changes:

```sh
xp2p server service start
xp2p client service start
```

On Linux run commands that change system state as root (use `sudo`).

## Switch modes (after install/deploy)

After the tunnel is installed, you can switch modes without re-running deploy/install. The commands update Desired inputs, record an apply request, and will restart the service automatically if it is currently running.

Switch the client to proxy mode (disables TUN):

```sh
xp2p client mode proxy
```

Switch the client to split-tunnel TUN mode:

```sh
xp2p client mode tun split
```

Switch the client to full-tunnel TUN mode:

```sh
xp2p client mode tun full
```

Switch the server mode:

```sh
xp2p server mode proxy
xp2p server mode tun
```

## Verify

From B:

```sh
xp2p ping 10.63.30.11
```

Advanced verification options:

- Use `--tunnel` to force the tunnel path and `--count <n>` to limit probes.
