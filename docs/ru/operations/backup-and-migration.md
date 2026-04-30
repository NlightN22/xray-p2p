# Резервная копия и перенос

Команды export/import переносят целиком снимок `CONFIG_ROOT` для выбранной роли.

Это удобно, чтобы переносить желаемые входные данные (Desired inputs) между машинами, хранить снимок для отката или клонировать настройку.

## Экспорт

Экспорт использует `CONFIG_ROOT` по умолчанию и пишет архив в текущую директорию:

```console
xp2p client export
xp2p server export
```

### Расширенные опции экспорта

- Экспорт из нестандартного корня конфигурации:

```console
xp2p client export --config-root <path>
xp2p server export --config-root <path>
```

- Выбор output path и формата:

```console
xp2p client export --output <path>
xp2p server export --output <path>
```

Формат определяется расширением output-файла:

- `.zip`
- `.tar.gz` (или `.tgz`)

Если `--output` не задан, xp2p выбирает формат по умолчанию (`.zip` на Windows, `.tar.gz` на не-Windows) и пишет `xp2p-<role>-backup-<timestamp>.<ext>` в текущую директорию.

## Импорт

```console
xp2p client import --input <archive>
xp2p server import --input <archive>
```

После импорта проверь статус сервиса и при необходимости перезапусти сервис соответствующей роли.

### Расширенные опции импорта

- Импорт в нестандартный корень конфигурации:

```console
xp2p client import --config-root <path> --input <archive>
xp2p server import --config-root <path> --input <archive>
```

Ограничения:

- Импорт принимает только желаемые входные данные (артефакты рантайма под `.state/` отклоняются).
- Символические ссылки (symlink) внутри архива не поддерживаются.
- Импорт записывает маркер `apply.request` для импортированной роли, чтобы сервис применил новые желаемые входные данные.
