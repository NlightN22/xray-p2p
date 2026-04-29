# Руководство по подключению XP2P

Это руководство устроено от простого к сложному. Начни с одиночного туннеля A–B,
а затем добавляй редиректы, обработку DNS и цепочки из нескольких узлов.

## Начни отсюда

- Установи xp2p: [Установка](getting-started/install.md)
- Создай первый туннель: [Первый туннель (A–B)](guides/single-tunnel.md)
- Добавь policy routing и правила по именам: [Редиректы в A–B](guides/redirects.md)
- Собери цепочку из нескольких узлов: [Цепочка (C2–B–A–C1)](guides/chain.md)
- Посмотри варианты (несколько клиентов, split/full tunnel, DNS): [Продвинутые варианты](guides/advanced.md)

## Как работает xp2p (flows)

- Handshake деплоя (что меняет и что не меняет): [Deploy flow](flows/deploy-flow.md)
- Механизм apply «желаемые входные данные → live-артефакты»: [Apply flow](flows/apply-flow.md)
- Как желаемые входные данные превращаются в `xray.json`: [Config compilation](flows/config-compilation.md)
- Как формируется runtime status: [Tunnel status logic](flows/tunnel-status.md)

## Понятия

- Термины, используемые в документации: [Терминология](getting-started/terminology.md)

## Объём лабораторного стенда

- OpenWrt-хосты для A и B.
- Alpine-гости для C1 и C2 (используются в сценарии с цепочкой).
- Команды используют пути и директории конфигурации по умолчанию.

## Условные обозначения

- A = server node, B = client node.
- C1, C2 = downstream-гости за NAT на B и A.
- Замени примерные IP, пользователей и пароли на свои значения.
