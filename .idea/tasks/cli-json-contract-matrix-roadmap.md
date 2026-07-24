# Дорожная карта полной JSON-контрактной матрицы CLI

## Назначение

Эта задача является навигационной для поэтапного покрытия всех JSON-enabled
leaf-команд `xp2p` исполняемыми контрактными тестами.

Исходный аудит и требования находятся в
`cli-json-output-contract-audit.md`. Текущий общий тест неизвестного флага
проверяет только error envelope Cobra и не считается success/error-покрытием
обработчика конкретной команды.

## Правила выполнения

- Этапы выполняются последовательно.
- Каждый этап завершается отдельным коммитом.
- Нельзя отмечать команду покрытой тестом `--help`, неизвестного флага или
  прямым тестом только JSON-wrapper.
- Success-сценарий обязан выполнить настоящий Cobra `RunE` команды.
- Разрешено подменять только внешние границы: сеть, service manager, firewall,
  runtime API, часы и генератор идентификаторов. Сам command handler и
  формирование его read model не подменяются.
- Все файловые Desired/Live fixtures изолируются во временном каталоге.
- Новая JSON leaf-команда после появления инфраструктуры матрицы сразу должна
  получить исполняемые success/error cases. Автоматический статус `pending`
  для новых команд запрещён.
- Полный Linux host-suite в рамках этих этапов запускается только по отдельному
  решению. Точечные Linux/WSL проверки разрешены.

## Этапы

1. [x] [Инфраструктура матрицы и реестр покрытия](cli-json-contract-matrix-01-infrastructure.md)
2. [x] [Read-only, list, status и empty-result команды](cli-json-contract-matrix-02-read-only.md)
3. [x] [Конфигурационные mutation-команды](cli-json-contract-matrix-03-mutations.md)
4. [ ] [Credentials, install, deploy, export и archives](cli-json-contract-matrix-04-credentials-artifacts.md)
5. [ ] [Interactive и service-команды](cli-json-contract-matrix-05-interactive-services.md)
6. [ ] [Linux, OpenWrt и Windows platform-specific команды](cli-json-contract-matrix-06-platforms.md)
7. [ ] [Итоговый аудит и включение обязательного gate](cli-json-contract-matrix-07-final-audit.md)

## Статусы существующих команд

До завершения седьмого этапа существующие JSON leaves могут иметь только один
из следующих явных статусов:

- `covered` — обязательные scenarios исполняются;
- `pending:<stage>` — команда закреплена за конкретным незавершённым этапом;
- `excluded:<reason>` — допустимо только для команды, которая не заявляет
  JSON-класс и уже имеет обоснованное исключение в output inventory.

Статус не заменяет тест. После завершения соответствующего этапа `pending`
должен быть удалён.

## Обязательный аудит после каждого этапа

После выполнения дочерней задачи необходимо вернуться в этот файл и проверить:

1. Реальное Cobra-дерево построено через `root.NewCommand`.
2. Для затронутых JSON leaves существуют реальные success и handler-error
   scenarios.
3. Проверены обязательные поля и исходные JSON-типы результата.
4. Где применимо, проверены empty result, credentials, Unicode/control
   characters, warnings, prompts и отсутствие ANSI.
5. Ошибка даёт ненулевой exit code, пустой stdout и один JSON-документ в
   stderr.
6. Human output не изменён без отдельного решения.
7. Audit inventory и contract-case registry не содержат missing/stale записей.
8. Выполнены проверки, указанные в дочерней задаче.
9. В отчёте перечислены покрытые command paths и оставшиеся
   `pending:<stage>`.
10. Чекбокс этапа отмечен только после выполнения всех критериев.

## Финальное условие

Работа завершена, когда итоговый meta-test автоматически сравнивает реальное
Cobra-дерево с исполняемым contract-case registry и падает для любой
JSON-enabled leaf-команды без полного обязательного набора scenarios. В
реестре не должно остаться статусов `pending`.

## Stage 1 result

- `covered`: 1 (`xp2p client list`)
- `pending:2`: 24
- `pending:3`: 52
- `pending:4`: 12
- `pending:5`: 13
- `pending:6`: 9
- Total JSON-enabled leaves: 111

The stage 1 meta-test builds the real Cobra tree through `root.NewCommand`,
compares it with both the output inventory and the executable contract-case
registry, and has explicit regression cases proving that missing and stale
entries fail. The frozen legacy baseline rejects `pending` for new commands.
Strict framing accepts exactly one JSON value followed by exactly one newline.
Executable warning and prompt scenarios verify clean streams and a real
non-interactive `RunE`.

## Stage 3 result

- `covered`: 52 mutation leaves
- `pending:3`: 0
- Each mutation executes its real Cobra `RunE` for staged success, handler
  failure, and legacy human mode.
- Success cases assert a concrete Desired or control-state change.
- Failure cases assert byte-for-byte state preservation.
- Mutation results contain `status`, `operation`, and the affected `entity`.
- Shared runtime-apply behavior remains verified by the client, server, and
  runtimeapply package suites for applied and rollback paths.
