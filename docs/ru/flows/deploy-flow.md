# Flow деплоя

Этот документ описывает flow деплоя в xp2p и то, как он связан с компиляцией
конфигурации и запуском сервисов.

## Область действия

- Применимо и к client deploy, и к server deploy.
- Следует правилам Apply Flow из [apply-flow.md](apply-flow.md).

## Ключевые правила

- Deploy обновляет Desired inputs и записывает `apply.request`.
- Deploy не запускает сервисы.
- Deploy может запустить временный экземпляр xray-core для проверки туннеля.
- Сервисы запускаются оператором явно после успешного deploy.
- При старте сервиса Desired inputs компилируются в live runtime artifacts до запуска.

## Обзор deploy

1. Client deploy генерирует deploy link и ждёт сервер.
2. Server deploy принимает зашифрованный manifest и обновляет Desired inputs:
   - `CONFIG_ROOT/xp2p-server.toml`
   - Опциональные JSON snippets в `CONFIG_ROOT/config-server/` (если они используются спецификацией deploy)
3. Server deploy записывает `apply.request` для роли server.
4. Client deploy получает ответ сервера и обновляет Desired inputs:
   - `CONFIG_ROOT/xp2p-client.toml`
   - Опциональные JSON snippets в `CONFIG_ROOT/config-client/` (если они используются спецификацией deploy)
5. Client deploy записывает `apply.request` для роли client.

## Временная проверка туннеля

Deploy может запустить xray-core с скомпилированной конфигурацией, чтобы проверить связность:

- Это временный runtime, используемый только во время deploy.
- Он не пишет live runtime artifacts.
- Он не запускает system service.
- Он завершается по окончании deploy.

Если сервис уже запущен, deploy не должен останавливать или перезапускать его. Deploy всё равно обязан
проверить туннель и не должен полагаться на service runtime для этой проверки. После успешного deploy оператор
должен перезапустить сервис, чтобы применить запрошенные изменения. Если сервис не запущен — вместо рестарта
нужно выполнить start.

## Deploy с live config и работающим сервисом

Deploy может выполняться поверх существующей установки с live config и активным сервисом:

- Desired inputs остаются источником истины для изменений deploy.
- Deploy обновляет Desired inputs и записывает `apply.request`.
- Работающий сервис должен продолжать работать и не должен быть перезапущен deploy.
- Временный xray-core для deploy используется только для проверки туннеля и не должен
  перезаписывать или переиспользовать service runtime.
- После успешного deploy:
  - Если сервис запущен, перезапустите его, чтобы применить изменения.
  - Если сервис не запущен, запустите его, чтобы применить изменения.

## Запуск сервисов после deploy

После успешного deploy оператор запускает сервисы явно:

- `xp2p server service start`
- `xp2p client service start`

При старте сервис:

1. Читает `apply.request`.
2. Компилирует Desired inputs в live runtime artifacts.
3. Удаляет `apply.request`.
4. Запускает runtime, используя live config.

Если скомпилированного live runtime config нет, сервис завершается со статусом
"no config available" и не запускает xray-core.

## Сигналы ошибок

Частые проблемы deploy:

- Невалидный Desired TOML / невалидные JSON snippets.
- Коллизии при merge (зарезервированные теги, неверный порядок правил, конфликты).
- Сервис не запущен после успешного deploy.

Если в логах deploy указаны невалидные Desired inputs или коллизии merge, исправьте Desired
файлы и снова запросите apply.

