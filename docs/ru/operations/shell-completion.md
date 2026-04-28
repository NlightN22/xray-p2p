# Автодополнение в shell

Большинство способов установки ставит shell completions автоматически:

- Debian/Ubuntu пакеты устанавливают completions в стандартные system locations.
- OpenWrt пакеты устанавливают completions в `/usr/share/...` completion directories.
- Windows MSI регистрирует PowerShell completion во время установки.

Используй `xp2p completion` только для archive-based installs, custom builds или troubleshooting.

Сгенерируй completion script для своей shell:

```sh
xp2p completion bash
xp2p completion zsh
xp2p completion fish
xp2p completion powershell
```

Загрузи сгенерированный скрипт стандартным для твоей shell способом (например через `~/.bashrc` / `~/.zshrc` profile).

