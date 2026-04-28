# Конфигурация

Эта страница описывает входные конфигурационные файлы и где xp2p их ищет. Для строгих правил apply в стиле Desired -> Live см. [Apply flow](../flows/apply-flow.md) и [Config compilation](../flows/config-compilation.md).

## Config root

По умолчанию xp2p использует `XP2P_CONFIG_ROOT`, если переменная задана, иначе — platform defaults (например `/etc/xp2p` на Linux/OpenWrt).

## Порядок загрузки (CLI)

Когда команда загружает конфигурацию, она merge'ит настройки в таком порядке:

1. Встроенные defaults
2. Опциональные config file(s)
3. Environment variables
4. CLI overrides

По умолчанию xp2p загружает `xp2p-client.toml` и `xp2p-server.toml` из config root; переопредели через `--config path/to/file`. Поддерживаются TOML и YAML.

Настройки маппятся 1:1 на environment variables через префикс `XP2P_` (`XP2P_SERVER_INSTALL_DIR`, `XP2P_CLIENT_SERVER_ADDRESS` и т.д.). Пример файла лежит в `config_templates/xp2p.example.yaml`.

## Global flags

У всех команд есть общие global flags: `--config`, `--log-level` (`debug|info|warn|error`), `--log-json`, `--version`.

Advanced / troubleshooting:

- Переопредели путь к config file через `--config path/to/file` для одноразового запуска.
- На Windows `xp2p client|server service start --log-level <level>` может сохранить `XP2P_LOG_LEVEL` в окружение сервиса для worker processes. Пакеты и сервисы всё равно запускаются с параметрами по умолчанию.

## Проверка версии Xray

Перед запуском runtime checks валидируют pinned версию xray. Override:

- `XP2P_XRAY_SKIP_VERSION_CHECK=1` (пропустить проверку)
- `XP2P_XRAY_ALLOW_MISMATCH=1` (warn и продолжить при несовпадении)

