import pytest

from tests.host.linux._external_subscription_3xui import (
    EXTENDED_MATRIX,
    FIXTURE_DIR,
    REMOTE_SUBSCRIPTION_URL,
    _failure_dump,
    _state_hashes,
    _wait_for_xp2p_traffic,
    cleanup_3xui,
    setup_3xui,
)

pytestmark = EXTENDED_MATRIX


OVERRIDE = f"{FIXTURE_DIR}/snapshot-override.sh"


@pytest.fixture
def running_3xui(aux_host):
    setup_3xui(aux_host)
    try:
        yield
    finally:
        cleanup_3xui(aux_host)


def test_3xui_snapshot_failures_and_optional_extensions(
    client_host, aux_host, xp2p_client_runner, running_3xui
):
    try:
        added = xp2p_client_runner(
            "client", "subscription", "add", "fixture", REMOTE_SUBSCRIPTION_URL, "--allow-http"
        )
        assert added.rc == 0, added.stderr
        started = xp2p_client_runner("client", "service", "start")
        assert started.rc == 0, started.stderr
        _wait_for_xp2p_traffic(client_host)
        baseline = _state_hashes(client_host)

        for mode in ("malformed", "oversized", "required"):
            _start_override(aux_host, mode)
            failed = xp2p_client_runner(
                "client", "subscription", "refresh", "fixture", "--allow-http"
            )
            assert failed.rc != 0
            assert "fixture-trojan-password" not in failed.stderr
            assert _state_hashes(client_host) == baseline
            _restore_3xui(aux_host)

        _start_override(aux_host, "optional")
        refreshed = xp2p_client_runner(
            "client", "subscription", "refresh", "fixture", "--allow-http"
        )
        assert refreshed.rc == 0, refreshed.stderr
        _restore_3xui(aux_host)
        restored = xp2p_client_runner(
            "client", "subscription", "refresh", "fixture", "--allow-http"
        )
        assert restored.rc == 0, restored.stderr
        restarted = xp2p_client_runner("client", "service", "restart")
        assert restarted.rc == 0, restarted.stderr
        _wait_for_xp2p_traffic(client_host)
    except AssertionError as error:
        pytest.fail(f"{error}\n{_failure_dump(client_host, aux_host)}")


def _start_override(aux_host, mode: str) -> None:
    captured = aux_host.run(f"sudo -n sh {OVERRIDE} capture")
    assert captured.rc == 0, captured.stderr
    stopped = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose stop")
    assert stopped.rc == 0, stopped.stderr
    started = aux_host.run(f"sudo -n sh {OVERRIDE} start {mode}")
    assert started.rc == 0, started.stderr


def _restore_3xui(aux_host) -> None:
    stopped = aux_host.run(f"sudo -n sh {OVERRIDE} stop")
    assert stopped.rc == 0, stopped.stderr
    started = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose start")
    assert started.rc == 0, started.stderr
    ready = aux_host.run(
        f"curl --fail --silent --retry 30 --retry-delay 1 --retry-connrefused "
        f"http://127.0.0.1:2096/sub/xp2pfixture2811"
    )
    assert ready.rc == 0, ready.stderr
