# Shell completion

Most installation methods install shell completions automatically:

- Debian/Ubuntu packages install completions into standard system locations.
- OpenWrt packages install completions into `/usr/share/...` completion directories.
- Windows MSI registers PowerShell completion during install.

Use `xp2p completion` only for archive-based installs, custom builds, or troubleshooting.

Generate a completion script for your shell:

```console
xp2p completion bash
xp2p completion zsh
xp2p completion fish
xp2p completion powershell
```

Use your shell's standard mechanism to load the generated script (for example your `~/.bashrc` / `~/.zshrc` profile).

