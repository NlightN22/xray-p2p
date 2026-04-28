# Логика статуса туннеля в runtime

Эта заметка описывает, как рассчитывается runtime-статус туннеля по client state файлу.

## Источник данных

Runtime status читает `C:\ProgramData\xp2p\xp2p-client.state.json` и использует:

- `tun_enabled`
- `mode`
- `runtime.tun.*`
- `runtime.routes.*`
- `runtime.socks_ready`
- `runtime.last_error`
- `runtime.timestamp`

Если файл отсутствует или `runtime` пустой, статус будет `Tun: Unknown`.

## Вывод статуса

### Готовность туннеля

- Если `tun_enabled=false` (proxy mode):
  - Ready, когда `runtime.socks_ready=true`
  - Pending, когда `runtime.socks_ready=false`
- Если `tun_enabled=true`:
  - `tun_ready = runtime.tun.ready`
  - `routes_ready = runtime.routes.full_applied || runtime.routes.redirect_applied`
  - Ready, когда и `tun_ready`, и `routes_ready` равны true
  - Pending иначе

### Метка routes

Логика статуса использует runtime flags с учётом текущего mode:

- `Proxy`, когда `tun_enabled=false`
- `Full`, когда `runtime.routes.full_applied=true`
- `Split`, когда `runtime.routes.redirect_applied=true`
- `Tun`, когда `tun_enabled=true` и ни один из route flags не true

### Ошибки

Если есть `runtime.last_error`, он добавляется в detail line.

## Примеры строк

- `Proxy: Ready (SOCKS)`
- `Tun: Ready | Routes: Full`
- `Tun: Pending | Routes: Split`
- `Tun: Unknown`

