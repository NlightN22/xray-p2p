# Apply Flow

Этот документ описывает, как xp2p применяет изменения конфигурации через строгий
механизм apply, принадлежащий сервису. Цель — сделать обновления конфигурации
атомарными, аудируемыми и безопасными для применения из сервисов без зависимости
от CLI flags.

## Ключевые файлы и директории

- `CONFIG_ROOT/.state/`
- `CONFIG_ROOT/.state/live/`
- `CONFIG_ROOT/.state/lkg/`
- `CONFIG_ROOT/.state/apply.request`
- `CONFIG_ROOT/.state/apply.error`
- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`
- `CONFIG_ROOT/config-client/`
- `CONFIG_ROOT/config-server/`

Файл `apply.request` — это триггер, который просит service layer скомпилировать
и применить Desired inputs.

## Акторы

- CLI, UI и ручные правки: обновляют Desired inputs и создают `apply.request`.
- Service layer (`xp2p run` или system service): читает `apply.request`,
  компилирует Desired inputs в runtime artifacts и выполняет очистку маркеров.

## Flow на верхнем уровне

1. Обновить Desired inputs (`xp2p-*.toml` и опциональные JSON snippets).
2. Записать `apply.request`.
3. Сервис обнаруживает запрос и компилирует runtime конфигурацию.
4. При успехе сервис удаляет `apply.request`, при ошибке пишет `apply.error`.
5. Runtime применяет OS routes и состояние TUN (только service layer).

## Desired inputs

Desired inputs всегда доступны для правки пользователем и лежат по стабильным путям:

- `CONFIG_ROOT/xp2p-client.toml`
- `CONFIG_ROOT/xp2p-server.toml`
- `CONFIG_ROOT/config-client/*.json` (опциональные snippets)
- `CONFIG_ROOT/config-server/*.json` (опциональные snippets)

xp2p читает эти inputs и компилирует их в итоговую конфигурацию Xray, которую использует runtime.

Рекомендованные имена snippets и точки вставки routing rules описаны в [Config compilation](config-compilation.md).

## Правила чтения и исключения

- Runtime behavior (service run, diagnostics, ping, OS routing) читает только live runtime
  artifacts и никогда не читает Desired inputs напрямую.
- При изменении Desired inputs нужно запросить apply через `apply.request`.
- Проверка во время deploy может запустить временный xray-core с конфигом, скомпилированным
  из Desired inputs, но она не должна писать в live и не должна обходить apply.

## Edit + rollback flow

Этот раздел описывает поток ручных правок, правок через CLI и rollback,
используя разделение Desired, Live и LKG. Запросы apply отслеживаются marker-файлом.

### Роли директорий

- Desired: редактируемые пользователем входные файлы конфигурации
  - `CONFIG_ROOT/xp2p-*.toml`
  - `config-client/*.json`, `config-server/*.json`
- Live: активная runtime-конфигурация
  - `CONFIG_ROOT/.state/live/`
- LKG: last known good snapshot (скрытый)
  - `CONFIG_ROOT/.state/lkg/`

Live и LKG хранят скомпилированные runtime artifacts (например `xray.json`) вместе с метаданными apply.

### Manual edit flow

1. Пользователь редактирует Desired файлы в `CONFIG_ROOT/` или `config-*/`.
2. Watchers дебаунсят серии изменений.
3. После стабилизации правок создаётся `apply.request`, чтобы триггернуть apply в сервисе.
4. Сервис компилирует Desired inputs и атомарно пишет live runtime artifacts.
5. При успехе предыдущий набор live artifacts сохраняется как LKG (опционально).

### CLI edit flow

1. CLI записывает изменения в Desired.
2. Создаётся `apply.request`, чтобы триггернуть apply в сервисе.
3. Сервис компилирует Desired inputs в live runtime artifacts.
4. При успехе предыдущий набор live artifacts сохраняется как LKG (опционально).

### Rollback flow

1. Apply падает (ошибка сервиса/xray/health checks).
2. Сервис восстанавливает live runtime artifacts из LKG (если доступно).
3. Записывается `apply.error` с request ID и причиной.
4. `apply.request` остаётся, чтобы оператор видел запрошенное изменение, но сервис
   пропускает повторные apply-попытки для того же request ID.
5. Сервис перезапускается, используя восстановленные live artifacts, и логирует ошибку.

## Deploy flow

Детали deploy flow (включая apply requests, временную проверку туннеля и требования к старту сервиса)
описаны в [Deploy flow](deploy-flow.md), чтобы избежать дублирования.

## Apply request

Trigger-файлы apply создаются по путям:

- `CONFIG_ROOT/.state/apply.request`
- `CONFIG_ROOT/.state/apply.error`

Внутри хранится роль (`client` или `server`) и request ID. Service process наблюдает
за этим файлом и рассматривает его как единственный источник истины для apply-работы.
При ошибке apply сервис пишет `apply.error` с тем же request ID и причиной.

## Service apply

При старте (или рестарте) сервис:

1. Читает `apply.request`.
2. Компилирует Desired inputs в live runtime artifacts.
3. Удаляет `apply.request`.
4. Пишет метаданные LKG при успехе (опционально).

Если apply падает, сервис логирует ошибку и оставляет `apply.request`, чтобы оператор
мог расследовать проблему или повторить попытку. Сервис не будет повторять apply для того же
request ID после записи `apply.error`; после исправления Desired inputs нужно создать новый
apply request.

## Routes и изменения ОС

Изменения ОС применяются только service layer:

- Создание TUN и назначение IP.
- Routes и full-tunnel изменения.
- DNS overrides (если включено).

CLI команды и UI flows обновляют Desired inputs и запрашивают apply. Они не трогают OS-level state напрямую.

## Контракт runtime OS state (TUN / routes / DNS)

Этот раздел определяет контракт service-owned OS state, чтобы избежать видимого "flapping" при рестартах.

### Владение и область

- Service layer владеет OS state (TUN, routes, DNS).
- Service layer обязан поддерживать OS state в соответствии с текущим Desired runtime mode.
- CLI/UI/ручные правки не должны напрямую менять OS state.

### Переходы, driven by mode

Переходы OS state определяются сменой mode, а не внутренними рестартами:

- Вход в full-tunnel (`client.tun_enabled=true` и `client.tun_mode=full`):
  - Заменить default routes на TUN интерфейс.
  - Добавить bypass routes для всех настроенных endpoints.
  - Применить DNS override на `client.dns_servers` (если настроено).
  - Держать full-tunnel активным, пока Desired остаётся в full-tunnel mode.
- Выход из full-tunnel (Desired уходит из full-tunnel):
  - Восстановить базовые default routes и убрать bypass routes.
  - Восстановить базовый DNS.
- Stop/uninstall сервиса:
  - Восстановить базовые routes/DNS (best effort) перед завершением.

### Семантика рестартов и отмены

Рестарты сервиса из-за `apply.request`, file watchers, health checks или crash recovery не должны вызывать
rollback routes/DNS, если Desired остаётся в full-tunnel mode.

- Отмена child-run (graceful restart) не является сменой mode.
- Rollback/restore разрешён только при явном stop, явной смене mode или при hard failures, требующих выхода из mode.

### Pending state и retry (Windows)

На Windows готовность TUN может быть задержана или нестабильна между рестартами (адаптер отключён, IPv4 отсутствует, DAD не preferred).
Когда Desired — full-tunnel, но адаптер не готов, runtime входит в pending state вместо rollback OS state.

- Pending state записывается в `CONFIG_ROOT/xp2p-client.tun-full.json` как `phase = "full_pending"` со стабильным `pending_reason`.
- Пока pending, routes и DNS override не применяются.
- Сервис повторяет попытки через рестарты с exponential backoff (2s, 4s, 8s, ... максимум 30s), пока адаптер не станет `up`/`preferred`.

## Переключение режимов

Смена режимов (split/full):

- Обновить `tun_enabled`, `tun_mode` и `full_tunnel_tag` в Desired TOML.
- Записать `apply.request`.
- Сервис компилирует конфиг и перезапускает runtime при необходимости.
- В full режиме повторное применение routes не переписывает конфиг; оно лишь
  обновляет OS routes.

## Частые причины ошибок

- Невалидный Desired TOML / невалидные JSON snippets.
- Коллизии при merge (зарезервированные теги, неверный порядок правил, конфликты).
- Сервис не запущен или apply request не обнаружен.

