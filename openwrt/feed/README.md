# xp2p OpenWrt feed

This directory hosts a minimal OpenWrt feed that builds the `xp2p` CLI into an
`.ipk`. The feed reuses the existing repository, so you do not need to maintain
yet another project when iterating on the Go sources.

## Quick start

1. **Clone the OpenWrt SDK / buildroot** for your target.
2. **Register the feed** by appending the following entry to
   `feeds.conf.default` (or `feeds.conf`):

   ```text
   src-git xp2p https://github.com/NlightN22/xray-p2p.git;main
   ```

3. **Update and install the feed**:

   ```bash
   ./scripts/feeds update xp2p
   ./scripts/feeds install xp2p
   ```

4. **Select the package** (`Network в†’ xp2p`) via `make menuconfig`, or enable it
   non-interactively with:

   ```bash
   echo "CONFIG_PACKAGE_xp2p=y" >> .config
   ```

5. **Build the ipk**:

   ```bash
   make package/xp2p/{clean,compile} V=sc
   ```

   The resulting package will be available under `bin/packages/<target>/xp2p/`.

## Customizing the source revision

By default the feed tracks the `main` branch (`PKG_SOURCE_VERSION:=main`). When
cutting a release you can override the revision from buildroot with:

```bash
./scripts/feeds update xp2p
./scripts/feeds install -a -p xp2p
make package/xp2p/download V=sc \
     XP2P_PKG_SOURCE_VERSION=<tag-or-commit>
```

Alternatively, edit `openwrt/feed/packages/utils/xp2p/Makefile` and set
`PKG_SOURCE_VERSION` to the desired tag/commit before running the build.

## Reproducible builds with Vagrant

The repository includes a Debian 12 Vagrant box that prepares the OpenWrt SDK
and mounts this workspace so you can build the ipk without touching your host.

1. Bring up the guest:

   ```bash
   cd infra/vagrant/debian13
   vagrant up deb13-server
   ```

2. Enter the VM when provisioning is done:

   ```bash
   vagrant ssh deb13-server
   ```

   The xp2p repository is mounted under `/srv/xray-p2p`, while the SDK lives in
   `/home/vagrant/openwrt-sdk`.

3. Build the ipk inside the guest:

   ```bash
   /srv/xray-p2p/tests/guest/scripts/build_openwrt_xp2p.sh
   ```

   The helper script injects the local feed (`src-link xp2p /srv/xray-p2p/openwrt/feed`),
   installs it, and compiles `xp2p` for every release architecture defined in
   `go/internal/buildtarget/target.go` (linux-amd64, linux-arm64,
   linux-mipsle-softfloat). Required SDKs are cached under
   `/home/vagrant/openwrt-sdk-<identifier>` on first use.

4. Collect the artifacts from the shared `./build/openwrt/<identifier>` folders
   inside the repository (e.g. `build/openwrt/linux-amd64/xp2p_*.ipk`). Use
   `vagrant ssh-config` + `scp` or `vagrant rsync` to copy them back to your host.

### Host Go toolchain

Install Go **1.23.x** on the host to keep `go.mod` compatible with the OpenWrt
SDK (the SDK currently ships Go 1.19). On Windows you can either install
`go1.23.3.windows-amd64.msi` or set `GOTOOLCHAIN=go1.23.3` (globally via
`setx` or in VS Code's `go.toolsEnvVars`) so every `go` command uses the same
toolchain as the module. Newer host versions will otherwise rewrite the module
to `go 1.24+`, which the SDK refuses to parse.

To smoke-test the build you can install the ipk inside the VM with `opkg install
./bin/packages/<target>/xp2p/xp2p_*.ipk` and run `xp2p --version`.

### Building for other OpenWrt targets

By default the helper builds all supported Linux targets. Use the following
environment variables if you need finer control:

- `XP2P_TARGETS` вЂ” comma-separated identifiers (e.g. `linux-amd64,linux-arm64`).
  The special value `all` (default) restores the multi-arch build.
- `XP2P_KEEP_CONFIG=1` вЂ” reuse the existing `.config` inside each SDK instead of
  generating a default one.
- `XP2P_OPENWRT_VERSION`, `XP2P_OPENWRT_MIRROR`, `XP2P_SDK_BASE`, and
  `XP2P_BUILD_ROOT` allow you to override the release, download mirror, SDK
  cache location, and destination for the resulting `.ipk` files respectively.

Example invocations inside the Vagrant guest:

```bash
# Build every release target and drop ipks into ./build/openwrt/*
/srv/xray-p2p/tests/guest/scripts/build_openwrt_xp2p.sh

# Only refresh the arm64 packages
XP2P_TARGETS=linux-arm64 /srv/xray-p2p/tests/guest/scripts/build_openwrt_xp2p.sh

# Keep a hand-crafted .config when rebuilding mipsle
XP2P_TARGETS=linux-mipsle-softfloat XP2P_KEEP_CONFIG=1 \
  /srv/xray-p2p/tests/guest/scripts/build_openwrt_xp2p.sh
```

Each run copies the resulting archives into `build/openwrt/<identifier>`, so you
can grab the artifacts from the host OS together with other release binaries.
