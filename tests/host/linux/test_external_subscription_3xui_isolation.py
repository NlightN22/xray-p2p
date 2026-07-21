import pytest

from tests.host.linux._external_subscription_3xui import (
    CLIENT_DESIRED,
    EXTENDED_MATRIX,
    FIXTURE_DIR,
    REMOTE_SUBSCRIPTION_URL,
    _assert_file_contains,
    _failure_dump,
    _wait_for_xp2p_traffic,
    cleanup_3xui,
    setup_3xui,
)

pytestmark = EXTENDED_MATRIX


SEED_ISOLATION = "/srv/xray-p2p/tests/guest/scripts/linux/seed_external_subscription_isolation.sh"
SECOND_LKG = "/etc/xp2p/.state/subscriptions/secondary.json"


@pytest.fixture
def isolation_environment(client_host, aux_host):
    setup_3xui(aux_host)
    try:
        yield
    finally:
        cleanup_3xui(aux_host)


def test_refresh_preserves_other_source_and_manual_resources(
    client_host, aux_host, xp2p_client_runner, isolation_environment
):
    try:
        primary = xp2p_client_runner(
            "client", "subscription", "add", "fixture", REMOTE_SUBSCRIPTION_URL, "--allow-http"
        )
        assert primary.rc == 0, primary.stderr
        secondary = xp2p_client_runner(
            "client",
            "subscription",
            "add",
            "secondary",
            REMOTE_SUBSCRIPTION_URL,
            "--allow-http",
        )
        assert secondary.rc == 0, secondary.stderr
        seeded = client_host.run(f"sudo -n sh {SEED_ISOLATION}")
        assert seeded.rc == 0, seeded.stderr
        started = xp2p_client_runner("client", "service", "start")
        assert started.rc == 0, started.stderr
        _wait_for_xp2p_traffic(client_host)
        secondary_before = client_host.run(f"sudo -n sha256sum {SECOND_LKG}")
        assert secondary_before.rc == 0, secondary_before.stderr

        rotated = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh rotate-credentials")
        assert rotated.rc == 0, rotated.stderr
        refreshed = xp2p_client_runner(
            "client", "subscription", "refresh", "fixture", "--allow-http"
        )
        assert refreshed.rc == 0, refreshed.stderr
        secondary_after = client_host.run(f"sudo -n sha256sum {SECOND_LKG}")
        assert secondary_after.stdout == secondary_before.stdout
        for marker in (
            "manual-isolation.example",
            "192.0.2.0/24",
            "manual-isolation-forward",
            "manual-isolation-channel",
        ):
            _assert_file_contains(client_host, CLIENT_DESIRED, marker)
        status = xp2p_client_runner("client", "subscription", "status")
        assert status.rc == 0, status.stderr
        assert "fixture" in status.stdout and "secondary" in status.stdout
        runtime = client_host.run(
            "sudo -n test -s /etc/xp2p/.state/live/config-client/xray.json "
            "&& pgrep -x xray >/dev/null"
        )
        assert runtime.rc == 0, runtime.stderr
    except AssertionError as error:
        pytest.fail(f"{error}\n{_failure_dump(client_host, aux_host)}")
