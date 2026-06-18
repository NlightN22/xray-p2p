# XRAY-p2p (Go)

XRAY-p2p delivers a cross-platform tunnel built on top of `xray-core`. The Go-based `xp2p` CLI owns server and client provisioning, state tracking, TLS assets, and helper routes on Windows, Linux, and OpenWrt so you no longer need to depend on the original shell automation.

## QuickStart

Install `xp2p`, run a deploy handshake from your client, then paste the generated deploy link into the server deploy command.

### 1) Install xp2p

<details>
<summary>OpenWrt:</summary>

```sh
wget -qO- https://nlightn22.github.io/xray-p2p/install-xp2p.sh | sh
```

</details>

<details>
<summary>Linux (Debian/Ubuntu amd64, .deb):</summary>

```sh
curl -fsSL -o xp2p.deb https://github.com/NlightN22/xray-p2p/releases/download/latest/xp2p-latest-linux-amd64.deb && sudo dpkg -i xp2p.deb
```

</details>

<details>
<summary>Linux (arm64, .tar.gz):</summary>

```sh
sudo install -d /opt/xp2p/bin /opt/xp2p/logs && curl -fsSL https://github.com/NlightN22/xray-p2p/releases/download/latest/xp2p-latest-linux-arm64.tar.gz | sudo tar -xz -C /opt/xp2p && sudo mv /opt/xp2p/xray /opt/xp2p/bin/xray && sudo ln -sf /opt/xp2p/xp2p /usr/local/bin/xp2p
```

</details>

<details>
<summary>Windows (amd64, MSI) (PowerShell):</summary>

```powershell
$msi = Join-Path $env:TEMP "xp2p.msi"; Invoke-WebRequest -Uri "https://github.com/NlightN22/xray-p2p/releases/download/latest/xp2p-latest-windows-amd64.msi" -OutFile $msi; Start-Process msiexec.exe -Wait -ArgumentList "/i `"$msi`""
```

</details>

<details>
<summary>Windows (x86, MSI) (PowerShell):</summary>

```powershell
$msi = Join-Path $env:TEMP "xp2p.msi"; Invoke-WebRequest -Uri "https://github.com/NlightN22/xray-p2p/releases/download/latest/xp2p-latest-windows-x86.msi" -OutFile $msi; Start-Process msiexec.exe -Wait -ArgumentList "/i `"$msi`""
```

</details>

### 2) Create a connection (Deploy handshake)

On your client machine:
```bash
xp2p client deploy --host <host>
```

On the server:
```bash
xp2p server deploy --link "<link>"
```

## What xp2p provides

- A single statically linked CLI (`xp2p`) with Cobra-based help, completions, doc generation, and a background diagnostics service.
- Server management covering installation, upgrades, TLS deployment, user provisioning, redirect/forward/reverse bridges, and `xray-core` log collection.
- Client management on Windows, Linux, and OpenWrt including endpoint installs from `trojan://` links, reverse portals, SOCKS autodiscovery, redirects, DNS-aware forwarding, and transparent NAT helpers.
- Remote deployment handshakes (`xp2p client deploy` + `xp2p server deploy`) that ship ready-to-use manifests over an encrypted link before bootstrapping both sides.
- Build tooling that emits per-OS packages together with vendor-supplied `xray` binaries, Windows MSI installers, Debian packages, and OpenWrt IPKs (publishable via `feeds.conf`).
- Pinned xray assets tracked in `go/internal/xray/pinned.json` with CI checksums and runtime version validation.

## Documentation

- Docs: [nlightn22.github.io/xray-p2p/docs](https://nlightn22.github.io/xray-p2p/docs/)

## Project layout and further docs

- `go/cmd/xp2p` and `go/internal/...` contain the CLI, installers, deploy logic, and state helpers.
- `config_templates/`, `distro/`, `installer/`, `openwrt/`, and `infra/` provide reference configs, bundled binaries, packaging manifests, and reproducible environments.
- Development, testing, and release guidance lives in [`CONTRIBUTING.md`](CONTRIBUTING.md), [`tests/README.md`](tests/README.md), and [`tests/TESTING_GUIDELINES.md`](tests/TESTING_GUIDELINES.md). Follow those docs for smoke tests, regression suites, and CI conventions.

## Acknowledgements

<details>
<summary>Open source projects</summary>

- Xray-core: https://github.com/XTLS/Xray-core
- Wintun (WireGuard): https://www.wintun.net/
- Cobra: https://github.com/spf13/cobra
- Koanf: https://github.com/knadh/koanf
- MkDocs: https://www.mkdocs.org/ and mkdocs-material: https://squidfunk.github.io/mkdocs-material/

</details>
