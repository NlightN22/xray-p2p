# Конфигурация

Эта страница описывает конфигурационные входные данные и где xp2p их ищет. Про строгие правила apply в стиле «желаемые входные данные → live-артефакты» см. [Apply flow](../flows/apply-flow.md) и [Config compilation](../flows/config-compilation.md).

## Корень конфигурации

По умолчанию xp2p использует `XP2P_CONFIG_ROOT`, если переменная задана. Иначе используются значения по умолчанию для платформы (например `/etc/xp2p` на Linux/OpenWrt).

## Порядок загрузки (CLI)

Когда команда загружает конфигурацию, она объединяет настройки в таком порядке:

1. Встроенные значения по умолчанию
2. Опциональные конфигурационные файлы
3. Переменные окружения
4. Переопределения из CLI

По умолчанию xp2p загружает `xp2p-client.toml` и `xp2p-server.toml` из корня конфигурации; переопредели через `--config path/to/file`. Поддерживаются TOML и YAML.

Настройки сопоставляются 1:1 с переменными окружения через префикс `XP2P_` (`XP2P_SERVER_INSTALL_DIR`, `XP2P_CLIENT_SERVER_ADDRESS` и т.д.). Пример файла лежит в `config_templates/xp2p.example.yaml`.

## Editor schema

В репозитории есть Taplo-compatible JSON schemas для TOML Desired inputs:

- `schemas/xp2p-client.schema.json`
- `schemas/xp2p-server.schema.json`

VS Code с Taplo extension читает `taplo.toml` из корня репозитория и применяет эти схемы к `xp2p-client.toml` и `xp2p-server.toml`. Схемы описывают только xp2p TOML inputs; они не валидируют generated Xray JSON artifacts.

## Глобальные флаги

У всех команд есть общие глобальные флаги: `--config`, `--log-level` (`debug|info|warn|error`), `--log-json`, `--version`.

Расширенные опции / устранение неполадок:

- Переопредели путь к файлу конфигурации через `--config path/to/file` для одноразового запуска.
- На Windows `xp2p client|server service start --log-level <level>` может сохранить `XP2P_LOG_LEVEL` в окружение сервиса для рабочих процессов. Пакеты и сервисы всё равно запускаются с параметрами по умолчанию.

## Проверка версии Xray

Перед запуском проверок в рантайме валидируется закреплённая версия `xray`. Переопределение:

- `XP2P_XRAY_SKIP_VERSION_CHECK=1` (пропустить проверку)
- `XP2P_XRAY_ALLOW_MISMATCH=1` (вывести предупреждение и продолжить при несовпадении)

## Xray asset files

Use `xray_assets` when routing rules reference xray-core `.dat` assets such as `geoip.dat`, `geosite.dat`, or `ext:<file>:...` files.

```toml
[[xray_assets.files]]
name = "geoip.dat"
url = "https://example.com/geoip.dat"

[[xray_assets.files]]
name = "geosite.dat"
url = "https://example.com/geosite.dat"
```

During service/run startup, xp2p checks the Live `xray.json` and the configured asset list before starting xray-core. Missing configured files are downloaded into the xray asset directory. Missing files found in routing rules but not configured fail startup with a clear preflight error.

Advanced:

- `xray_assets.stale_after` sets a shared refresh interval for all configured files.
- `xray_assets.files[].stale_after` overrides the shared refresh interval for one file.
- Empty or `0` `stale_after` disables periodic refresh while still requiring the file to exist.
- xp2p uses `XRAY_LOCATION_ASSET` when it is set; otherwise it uses the directory that contains the resolved `xray` binary.
