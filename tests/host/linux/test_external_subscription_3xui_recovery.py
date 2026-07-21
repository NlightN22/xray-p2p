import pytest

from tests.host.linux._external_subscription_3xui import (
    CLIENT_DESIRED,
    CLIENT_LIVE,
    CLIENT_SUBSCRIPTION_LKG,
    EXTENDED_MATRIX,
    FIXTURE_DIR,
    REMOTE_SUBSCRIPTION_URL,
    _assert_file_contains,
    _assert_offer_count,
    _failure_dump,
    _fetch_subscription,
    _wait_for_file_contains,
    _wait_for_xp2p_traffic,
    cleanup_3xui,
    setup_3xui,
)

pytestmark = EXTENDED_MATRIX


@pytest.fixture
def running_3xui(aux_host):
    setup_3xui(aux_host)
    try:
        yield
    finally:
        cleanup_3xui(aux_host)


def test_refresh_while_stopped_updates_desired_and_lkg_only(
    client_host, aux_host, xp2p_client_runner, running_3xui
):
    try:
        _add_and_start(xp2p_client_runner)
        _wait_for_xp2p_traffic(client_host)
        stopped = xp2p_client_runner("client", "service", "stop")
        assert stopped.rc == 0, stopped.stderr
        rotated = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh rotate-credentials")
        assert rotated.rc == 0, rotated.stderr
        refreshed = xp2p_client_runner(
            "client", "subscription", "refresh", "fixture", "--allow-http"
        )
        assert refreshed.rc == 0, refreshed.stderr
        _assert_file_contains(client_host, CLIENT_DESIRED, "rotated-trojan-password")
        _assert_file_contains(client_host, CLIENT_SUBSCRIPTION_LKG, "rotated-trojan-password")
        _assert_file_contains(client_host, CLIENT_LIVE, "fixture-trojan-password")

        started = xp2p_client_runner("client", "service", "start")
        assert started.rc == 0, started.stderr
        _wait_for_file_contains(client_host, CLIENT_LIVE, "rotated-trojan-password")
        _wait_for_xp2p_traffic(client_host)
    except AssertionError as error:
        pytest.fail(f"{error}\n{_failure_dump(client_host, aux_host)}")


def test_refresh_recovers_persisted_source_after_client_and_3xui_restart(
    client_host, aux_host, xp2p_client_runner, running_3xui
):
    try:
        _add_and_start(xp2p_client_runner)
        _wait_for_xp2p_traffic(client_host)
        client_restart = xp2p_client_runner("client", "service", "restart")
        assert client_restart.rc == 0, client_restart.stderr
        panel_restart = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose restart")
        assert panel_restart.rc == 0, panel_restart.stderr
        _fetch_subscription(aux_host)
        _assert_offer_count(xp2p_client_runner, 2)
        _assert_file_contains(client_host, CLIENT_SUBSCRIPTION_LKG, "fixture-trojan-password")

        rotated = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh rotate-credentials")
        assert rotated.rc == 0, rotated.stderr
        refreshed = xp2p_client_runner(
            "client", "subscription", "refresh", "fixture", "--allow-http"
        )
        assert refreshed.rc == 0, refreshed.stderr
        _assert_file_contains(client_host, CLIENT_DESIRED, "rotated-trojan-password")
        _assert_file_contains(client_host, CLIENT_LIVE, "rotated-trojan-password")
        _wait_for_xp2p_traffic(client_host)
    except AssertionError as error:
        pytest.fail(f"{error}\n{_failure_dump(client_host, aux_host)}")


def _add_and_start(runner) -> None:
    added = runner(
        "client", "subscription", "add", "fixture", REMOTE_SUBSCRIPTION_URL, "--allow-http"
    )
    assert added.rc == 0, added.stderr
    started = runner("client", "service", "start")
    assert started.rc == 0, started.stderr
