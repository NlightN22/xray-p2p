# Backup и перенос

Export/import переносит целиком snapshot `CONFIG_ROOT` для выбранной роли.

Это полезно, чтобы переносить Desired inputs между машинами, хранить rollback snapshot или клонировать настройку.

## Export

Export использует `CONFIG_ROOT` по умолчанию и пишет архив в текущую директорию.

```sh
xp2p client export
xp2p server export
```

## Advanced export

- Экспорт из нестандартного config root:

```sh
xp2p client export --config-root <path>
xp2p server export --config-root <path>
```

- Выбор output path и формата:

```sh
xp2p client export --output <path>
xp2p server export --output <path>
```

Поддерживаемые форматы определяются расширением output-файла:

- `.zip`
- `.tar.gz` (или `.tgz`)

Если `--output` не задан, xp2p выбирает формат по умолчанию (`.zip` на Windows, `.tar.gz` на не-Windows) и пишет `xp2p-<role>-backup-<timestamp>.<ext>` в текущую директорию.

## Import

```sh
xp2p client import --input <archive>
xp2p server import --input <archive>
```

После import проверь status сервиса и при необходимости перезапусти service для роли.

## Advanced import

- Импорт в нестандартный config root:

```sh
xp2p client import --config-root <path> --input <archive>
xp2p server import --config-root <path> --input <archive>
```

Notes:

- Import принимает только Desired inputs (runtime artifacts в `.state/` отклоняются).
- Symlinks внутри bundle не поддерживаются.
- Import пишет marker `apply.request` для импортированной роли, чтобы сервис мог пере-применить новые Desired inputs.

