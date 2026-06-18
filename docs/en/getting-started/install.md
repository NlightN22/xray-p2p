# Install

This page covers installation basics per platform. For the fastest end-to-end setup, start with [First tunnel (A-B)](../guides/single-tunnel.md).

## OpenWrt

One-line installer (auto-detects release/arch, adds feed/signing key, installs package):

```console
wget -qO- https://nlightn22.github.io/xray-p2p/install-xp2p.sh | sh
```

- Services land under `/etc/init.d/xp2p-client` and `/etc/init.d/xp2p-server` and run `xp2p client|server service run` with default flags.
- Manage lifecycle: `service xp2p-client start|stop|restart|status` or `xp2p client service start|status`; logs live in `/var/log/xp2p/`.
- On a live OpenWrt system, `opkg install` triggers `default_postinst`, which enables and starts init.d services immediately.
- Remove: `opkg remove xp2p` (stops services and removes the package). To purge, delete `/etc/xp2p`, `/var/log/xp2p`, and the init scripts.

Optional manual feed setup:

```console
echo "src-git xp2p https://github.com/NlightN22/xray-p2p.git;main" >> /etc/opkg/customfeeds.conf && opkg update && opkg install xp2p
```

From a local IPK:

```console
opkg install /tmp/xp2p_<version>_<arch>.ipk
```

## Linux (Debian/Ubuntu packages)

- Grab the `.deb` for your arch from the Release (`xp2p_<version>_amd64.deb`, `xp2p_<version>_arm64.deb`, `xp2p_<version>_armhf.deb`, `xp2p_<version>_386.deb`).
- Install: `sudo dpkg -i xp2p_<version>_<arch>.deb || sudo apt-get -f install`.
- Binaries land under `/usr/bin/xp2p`, bundled `xray` under `/etc/xp2p/bin/xray`, configs under `/etc/xp2p/config-{client,server}`, logs under `/var/log/xp2p`.
- Services: `systemctl enable --now xp2p-client xp2p-server` (wrap `xp2p client|server service run` with default flags).
- Remove: `sudo dpkg -r xp2p`; purge data with `sudo dpkg -P xp2p`.

## Windows (MSI)

- Download `xp2p-<version>-windows-amd64.msi` (or the `.zip` archive).
- Install MSI with standard commands:

```powershell
msiexec /i xp2p-<version>-windows-amd64.msi
msiexec /x xp2p-<version>-windows-amd64.msi
```

- Services `xp2p-client` and `xp2p-server` wrap `xp2p client|server service run`; manage via `xp2p client service start|stop|status` or the Services snap-in.
- Logs: `C:\Program Files\xp2p\logs\<role>\`.
- Service management (start/stop/status) is available to Builtin Users without admin elevation.
- Tray controller `ui-xp2p` ships with the MSI and auto-starts via the current user's Run key; disable with `XP2P_UI_AUTOSTART=0`.

Need to build the Windows MSI on a Windows host? Follow [`scripts/build/README.md`](https://github.com/NlightN22/xray-p2p/blob/main/scripts/build/README.md).

## Archives (release tar.gz)

Release archives contain `xp2p` together with bundled `xray`. Keep both binaries together and add `xp2p` to your `PATH`.

## Quick start (manual install)

These examples use `xp2p server install` + `xp2p client install` (no deploy handshake). For the deploy handshake path, see [First tunnel (A-B)](../guides/single-tunnel.md).

### OpenWrt

```console
opkg update && opkg install xp2p
xp2p server install --host edge.example.com
xp2p client install --link "<LINK_FROM_SERVER_INSTALL>"
xp2p server service start
xp2p client service start
xp2p server state
```

### Linux

```console
sudo dpkg -i xp2p_<version>_amd64.deb || sudo apt-get -f install
sudo xp2p server install --host edge.example.com
sudo xp2p client install --link "<LINK_FROM_SERVER_INSTALL>"
sudo systemctl enable --now xp2p-server xp2p-client
xp2p server state
```

### Windows

```powershell
msiexec /i xp2p-<version>-windows-amd64.msi
xp2p server install --host edge.example.com
xp2p client install --link "<LINK_FROM_SERVER_INSTALL>"
xp2p client service start
xp2p server service start
xp2p client reverse list
```

Advanced options:

- Custom service port: add `--port <port>` to `xp2p server install` and use the generated link on the client.
- Self-signed TLS: pass `--allow-insecure` to `xp2p client install`.
- Manual client fields (no link): `xp2p client install --host <host> --user <user> --password <password>`.
