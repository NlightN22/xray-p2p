# Linux Xray bundles

This tree mirrors the Windows bundles under `distro/windows/bundle`. Drop the
pre-built Linux `xray` binaries into the matching architecture folders whenever
you want to ship them alongside xp2p.

Available placeholders:

- `bundle/x86_64/xray` - Linux 64-bit (aka amd64 / x86_64)
- `bundle/x86/xray` - Linux 32-bit (aka 386)
- `bundle/arm64/xray` - ARM64 / AArch64
- `bundle/armhf/xray` - 32-bit ARM hard-float
- `bundle/mipsel/xray` - MIPS32 little-endian
- `bundle/mips64el/xray` - MIPS64 little-endian
- `bundle/riscv64/xray` - RISC-V 64

Need another target? Create a sibling folder under `bundle/` and place the
binary there using the same convention.

These folders stay empty in Git via `.gitkeep`, so feel free to replace them with
real binaries locally. OpenWrt targets are intentionally excluded to avoid
bloating xp2p artifacts, so keep installing the upstream packages there instead.
