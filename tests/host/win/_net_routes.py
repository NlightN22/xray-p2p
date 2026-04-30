import base64
import json
from collections.abc import Iterable

from testinfra.host import Host


def get_interface_index(host: Host, interface_name: str) -> int:
    from . import env as _env

    result = _env.run_guest_script(
        host,
        "scripts/get_net_adapter_index.ps1",
        InterfaceName=interface_name,
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to query interface index.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    value = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not value:
        raise RuntimeError(f"No interface index returned for {interface_name!r}")
    try:
        return int(value[-1])
    except ValueError as exc:
        raise RuntimeError(f"Unexpected interface index output: {result.stdout!r}") from exc


def get_net_routes(
    host: Host,
    destination_prefix: str,
    interface_index: int | None = None,
) -> list[dict]:
    from . import env as _env

    parameters: dict[str, object] = {
        "DestinationPrefix": destination_prefix,
    }
    if interface_index is not None:
        parameters["InterfaceIndex"] = str(interface_index)
    result = _env.run_guest_script(
        host,
        "scripts/get_net_routes.ps1",
        **parameters,
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to query net routes.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    payload = (result.stdout or "").strip()
    if not payload:
        return []
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Unexpected net routes output: {payload!r}") from exc
    if data is None:
        return []
    if isinstance(data, dict):
        return [data]
    if isinstance(data, list):
        return data
    raise RuntimeError(f"Unexpected net routes output type: {type(data).__name__}")


def remove_tun_adapters(host: Host, adapter_names: Iterable[str]) -> None:
    from . import env as _env

    names = [str(name) for name in adapter_names if str(name).strip()]
    if not names:
        return
    payload = base64.b64encode(json.dumps(names).encode("utf-8")).decode("ascii")
    result = _env.run_guest_script(
        host,
        "scripts/remove_tun_adapters.ps1",
        NamesBase64=payload,
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to remove TUN adapters.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

