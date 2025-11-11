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

4. **Select the package** (`Network → xp2p`) via `make menuconfig`, or enable it
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
