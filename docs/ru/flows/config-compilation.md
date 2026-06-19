# Компиляция конфигурации

Этот документ описывает, как xp2p превращает операторскую конфигурацию
(TOML + опциональные JSON-фрагменты) в финальную конфигурацию времени выполнения
для `xray-core`.

Цели:

- Держать редактируемые человеком входные данные небольшими и стабильными.
- Разрешить безопасные расширения пользователя без редактирования сгенерированных файлов.
- Получать детерминированный, валидируемый финальный JSON-файл конфигурации Xray.

## Источники истины

xp2p рассматривает конфигурацию как набор слоёв входных данных.

### Желаемые входные данные (редактирует оператор)

Управляемое поведение настраивается через TOML и может редактироваться вручную или через CLI/UI:

- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`

Расширения пользователя задаются JSON-фрагментами в каталогах роли:

- `CONFIG_ROOT/config-client/*.json`
- `CONFIG_ROOT/config-server/*.json`

xp2p читает эти JSON-файлы и объединяет их с финальной конфигурацией Xray. xp2p
не должен их переписывать.

### Артефакты сборки (выходы рантайма)

xp2p компилирует входные данные в финальный JSON-файл конфигурации Xray, который используется
во время работы:

- `CONFIG_ROOT/.state/live/config-client/xray.json`
- `CONFIG_ROOT/.state/live/config-server/xray.json`

Эти файлы генерируются сервисным слоем применения или успешным применением
команды CLI во время работы. Их нельзя редактировать вручную.

## Структура каталогов

- Желаемые входные данные
  - `CONFIG_ROOT/xp2p-*.toml`
  - `CONFIG_ROOT/config-*/` (JSON-фрагменты)
- Состояние сервиса и артефакты времени выполнения
  - `CONFIG_ROOT/.state/apply.request`
  - `CONFIG_ROOT/.state/apply.error`
  - `CONFIG_ROOT/.state/live/config-*/xray.json`
  - `CONFIG_ROOT/.state/lkg/config-*/xray.json` (опционально)
  - `CONFIG_ROOT/audit.log`

Директория `.state` хранит метаданные применения и скомпилированные артефакты
времени выполнения. Она не зеркалирует желаемые входные данные.

## Конвейер компиляции

При применении или старте сервиса каждая роль (`client` или `server`) выполняет:

1. Загрузить Desired TOML (`CONFIG_ROOT/xp2p-*.toml`).
2. Загрузить Desired JSON-фрагменты из `CONFIG_ROOT/config-*/` (если есть).
3. Построить управляемую базовую конфигурацию Xray из TOML (точки подключения,
   маршрутизация, входящие слушатели, журналы, обратные мосты и т.д.).
4. Объединить пользовательские JSON-фрагменты с управляемой базой по
   детерминированным правилам.
5. Проверить финальную конфигурацию (структура, коллизии зарезервированных
   тегов, обязательные секции).
6. Атомарно записать финальную конфигурацию в `.state/live/config-*/xray.json`.
7. Запустить/перезапустить Xray, используя только финальный JSON файл.

## Кандидатная компиляция для применения во время работы

Некоторые команды CLI меняют только ресурсы Xray, которые закреплённый gRPC API
Xray может обновить и проверить без перезапуска `xray-core`. Такие команды
используют кандидатную компиляцию до записи постоянного состояния:

1. Загрузить текущие желаемые TOML-файлы и JSON-фрагменты.
2. Применить запрошенное изменение к кандидату в памяти.
3. Скомпилировать и проверить кандидата теми же правилами, что и сервисное
   применение.
4. Если запущенный Xray доступен, применить кандидата через API Xray и проверить
   результат во время работы.
5. При успехе записать соответствующие Live-артефакты и сохранить
   соответствующие желаемые входные данные.
6. Если сервис остановлен или запущенная Live-конфигурация недоступна, сохранить
   только желаемые входные данные и не менять Live.
7. Если применение через API или проверка завершается ошибкой, оставить Desired
   и Live без изменений.

Этот путь держит успешное состояние запущенного Xray, Live-артефакты и желаемые
входные данные согласованными без создания `apply.request`. Изменения уровня ОС,
такие как TUN, маршруты, DNS, файрвол и nftables, остаются во владении сервиса
и не входят в кандидатное применение во время работы.

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
