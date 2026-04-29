# Компиляция конфигурации

Этот документ описывает, как xp2p превращает операторскую конфигурацию (TOML + опциональные JSON snippets)
в финальную runtime-конфигурацию `xray-core`.

Цели:

- Держать human-editable inputs небольшими и стабильными.
- Разрешить безопасные расширения пользователя без редактирования сгенерированных файлов.
- Получать детерминированный, валидируемый финальный Xray JSON config.

## Источники истины

xp2p рассматривает конфигурацию как набор слоёв входных данных.

### Желаемые входные данные (редактирует оператор)

Managed поведение настраивается через TOML и может редактироваться вручную или через CLI/UI:

- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`

Расширения пользователя задаются JSON snippets в role-директориях:

- `CONFIG_ROOT/config-client/*.json`
- `CONFIG_ROOT/config-server/*.json`

xp2p читает эти JSON файлы и merge'ит их в финальную конфигурацию Xray. xp2p не должен их переписывать.

### Артефакты сборки (выходы рантайма)

xp2p компилирует inputs в финальный Xray JSON файл, который используется в runtime:

- `CONFIG_ROOT/.state/live/config-client/xray.json`
- `CONFIG_ROOT/.state/live/config-server/xray.json`

Эти файлы генерируются service/apply layer и не должны редактироваться вручную.

## Структура каталогов

- Желаемые входные данные
  - `CONFIG_ROOT/xp2p-*.toml`
  - `CONFIG_ROOT/config-*/` (JSON snippets)
- Service state и runtime artifacts
  - `CONFIG_ROOT/.state/apply.request`
  - `CONFIG_ROOT/.state/apply.error`
  - `CONFIG_ROOT/.state/live/config-*/xray.json`
  - `CONFIG_ROOT/.state/lkg/config-*/xray.json` (опционально)
  - `CONFIG_ROOT/audit.log`

Директория `.state` хранит метаданные apply и скомпилированные runtime artifacts. Она не зеркалирует желаемые входные данные.

## Конвейер компиляции

Для каждой роли (client/server) service/apply layer выполняет:

1. Загрузить Desired TOML (`CONFIG_ROOT/xp2p-*.toml`).
2. Загрузить Desired JSON snippets из `CONFIG_ROOT/config-*/` (если есть).
3. Построить managed base Xray config из TOML (endpoints, routing, входящие слушатели, logs, reverse bridges и т.д.).
4. Merge'ить user snippets в managed base по детерминированным правилам.
5. Валидировать финальный config (структура, коллизии reserved tags, обязательные секции).
6. Атомарно записать финальный config в `.state/live/config-*/xray.json`.
7. Запустить/перезапустить Xray, используя только финальный JSON файл.

## Правила объединения

xp2p merge'ит JSON snippets в managed base по role-специфичным правилам. Merge явный и детерминированный.

## Поддерживаемые файлы расширений

JSON snippets лежат в:

- `CONFIG_ROOT/config-client/`
- `CONFIG_ROOT/config-server/`

Все файлы опциональны. Для удобства xp2p поставляет пустые шаблоны в:

- `config_templates/extensions/config-client/`
- `config_templates/extensions/config-server/`

### Клиент

- `routing.rules.after-xp2p-system.json`
  - Формат: `{ "rules": [ ... ] }`
  - Вставляется после managed endpoint-bypass + system glue rules.
  - Используйте для high-priority правил, которые должны "перебивать" redirects/forwards.
- `routing.rules.after-xp2p-managed.json`
  - Формат: `{ "rules": [ ... ] }`
  - Вставляется после всех managed правил и перед full-tunnel rule (если включено).
  - Используйте для дополнительных правил, которые не должны переопределять core safety rules.
- `inbounds.append.json`
  - Формат: `{ "inbounds": [ ... ] }`
  - Добавляется в конец managed inbounds.
- `outbounds.append.json`
  - Формат: `{ "outbounds": [ ... ] }`
  - Добавляется в конец managed outbounds.

### Сервер

- `routing.rules.after-xp2p-system.json` (то же значение, что и у client)
- `routing.rules.after-xp2p-managed.json` (то же значение, что и у client)
- `inbounds.append.json` (добавить к managed inbounds)
- `outbounds.append.json` (добавить к managed outbounds)

### Зарезервированное пространство имён

xp2p резервирует идентификаторы, которыми он управляет (tags, remarks, internal routing domains).

По умолчанию зарезервированы:

- `proxy-*` (endpoint outbounds)
- `*.rev` (reverse bridge domains)
- `xp2p-*` (internal tags и glue components)

User snippets не должны создавать коллизии с reserved identifiers, если только не включён явный override mode.

### Порядок правил маршрутизации

Финальный порядок routing rules стабилен:

1. Managed endpoint bypass rules (для безопасной доступности туннеля).
2. Managed system rules (reverse/marker glue, internal responders).
3. Managed redirect/forward rules.
4. User rules (опционально), вставляемые в extension points (см. Supported Extension Files выше).
5. Managed full-tunnel rule (если включено).

Если user routing snippets присутствуют, они вставляются только в настроенные extension points.

### Исходящие, входящие и другие секции

User snippets могут добавлять дополнительные объекты, но xp2p-managed объекты остаются authoritative.

Типовые extension points:

- Дополнительные `inbounds`
- Дополнительные `outbounds`
- Дополнительные `routing.rules`
- Секции DNS и policy (если предоставляет Xray)

Для безопасности xp2p отклоняет:

- Попытки удалить managed objects.
- Попытки модифицировать reserved tags без override mode.
- Попытки добавить объекты с невалидными или конфликтующими tags.

## Команды для инспекции

xp2p предоставляет команды инспекции, которые не меняют runtime state.

### Рендер финального Xray JSON

Отрендерить ровно тот JSON, который будет использован в runtime:

```bash
xp2p client render xray --live --output -
xp2p server render xray --live --output -
```

Отрендерить конфигурацию, скомпилированную из желаемых входных данных (без применения):

```bash
xp2p client render xray --desired --output -
xp2p server render xray --desired --output -
```

### Диагностический пакет

Собрать self-contained архив для troubleshooting:

```bash
xp2p client debug bundle --output /tmp/xp2p-client-debug.zip
xp2p server debug bundle --output /tmp/xp2p-server-debug.zip
```

Bundle включает:

- Желаемые входные данные (`xp2p-*.toml` и `config-*/` snippets)
- `apply.request` / `apply.error` (если есть)
- финальный `xray.json`
- `audit.log`
- service logs в `XP2P_LOG_ROOT`

## Операционные заметки

- Операторы редактируют только TOML и JSON snippets.
- Service/apply layer компилирует и валидирует configs в `xray.json`.
- Runtime процессы всегда используют только live compiled `xray.json`.
