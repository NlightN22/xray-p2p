from __future__ import annotations

import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

pytestmark = [pytest.mark.host, pytest.mark.linux]

CLIENT_TUN = "xp2pc"
CLIENT_ADDR = "198.18.0.1/30"
FULL_STATE_FILE = helpers.CONFIG_ROOT / "xp2p-client.tun-full.json"

SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 1.5


def _xp2p(host, *args: str, check: bool = False):
    result = openwrt_env.run_xp2p(host, *args)
    if check and result.rc != 0:
        pytest.fail(
            f"xp2p command failed (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _read_full_state(host) -> dict:
    if not helpers.path_exists(host, FULL_STATE_FILE):
        return {}
    try:
        return helpers.read_json(host, FULL_STATE_FILE)
    except Exception:
        return {}


def _full_phase(state: dict) -> str:
    return str(state.get("phase") or "").strip().lower()


def _full_enabled(state: dict) -> bool | None:
    value = state.get("enabled")
    if isinstance(value, bool):
        return value
    return None


def _wait_for_full_applied(host, timeout: float = 75.0) -> dict:
    deadline = time.time() + timeout
    last = {}
    while time.time() < deadline:
        last = _read_full_state(host)
        if _full_enabled(last) is True and _full_phase(last) == "full_applied":
            return last
        time.sleep(POLL_INTERVAL)
    raise AssertionError(f"full-tunnel did not reach full_applied. Last state: {last}")


def _default_routes(host) -> list[str]:
    result = host.run("ip -4 route show default 2>/dev/null || true")
    return [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]


def _assert_default_route_via_tun(host, tun: str, timeout: float = 30.0) -> None:
    deadline = time.time() + timeout
    last: list[str] = []
    needle = f"dev {tun}"
    while time.time() < deadline:
        last = _default_routes(host)
        if any(needle in line for line in last):
            return
        time.sleep(POLL_INTERVAL)
    raise AssertionError(f"Expected default route via {tun}. Last routes: {last}")


def _assert_tun_addr(host, name: str, addr: str, timeout_seconds: int | None = None) -> None:
    args = [name, addr]
    if timeout_seconds is not None:
        args.append(str(timeout_seconds))
    result = openwrt_env.run_guest_script(host, "scripts/openwrt/assert_tun_addr.sh", *args)
    assert result.rc == 0, (
        "TUN address check failed.\n" f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_full_tunnel_survives_internal_service_restart(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    runner = lambda *cmd, check=False: _xp2p(openwrt_host, *cmd, check=check)

    helpers.cleanup_client_install(openwrt_host, runner)
    openwrt_env._stop_xp2p_services(openwrt_host)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.70.10",
            "--user",
            "tun-full-openwrt@example.com",
            "--password",
            "tun-full-openwrt-pass",
            "--force",
            check=True,
        )
        runner("client", "service", "start", check=True)
        helpers.wait_for_service_state(openwrt_host, "client", expected_active=True, timeout_seconds=SERVICE_TIMEOUT)
        helpers.wait_for_apply_request_clear(openwrt_host, timeout_seconds=SERVICE_TIMEOUT)

        runner(
            "client",
            "mode",
            "tun",
            "full",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.70.10",
            "--quiet",
            check=True,
        )
        helpers.wait_for_apply_request_clear(openwrt_host, timeout_seconds=SERVICE_TIMEOUT)
        _wait_for_full_applied(openwrt_host)
        _assert_tun_addr(openwrt_host, CLIENT_TUN, CLIENT_ADDR, timeout_seconds=30)
        _assert_default_route_via_tun(openwrt_host, CLIENT_TUN, timeout=45.0)

        helpers.write_apply_request(openwrt_host, "client")
        helpers.wait_for_apply_request(openwrt_host, timeout_seconds=10.0, poll_interval=0.5)
        helpers.wait_for_apply_request_clear(openwrt_host, timeout_seconds=SERVICE_TIMEOUT)

        state = _read_full_state(openwrt_host)
        assert _full_enabled(state) is True, f"Expected full-tunnel enabled after internal restart. State: {state}"
        assert _full_phase(state) in {"full_applied", "full_pending"}, f"Unexpected full-tunnel phase: {state}"
        _assert_tun_addr(openwrt_host, CLIENT_TUN, CLIENT_ADDR, timeout_seconds=30)
        _assert_default_route_via_tun(openwrt_host, CLIENT_TUN, timeout=45.0)
    except Exception:
        helpers.dump_failure_state(openwrt_host, "tun_mode_full_internal_restart")
        raise
    finally:
        runner("client", "service", "stop")
        helpers.cleanup_client_install(openwrt_host, runner)
