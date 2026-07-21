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


INJECT = "/srv/xray-p2p/tests/guest/scripts/linux/external_subscription_failure_injection.sh"


@pytest.fixture
def running_subscription(client_host, aux_host, xp2p_client_runner):
    setup_3xui(aux_host)
    added = xp2p_client_runner(
        "client", "subscription", "add", "fixture", REMOTE_SUBSCRIPTION_URL, "--allow-http"
    )
    assert added.rc == 0, added.stderr
    started = xp2p_client_runner("client", "service", "start")
    assert started.rc == 0, started.stderr
    _wait_for_xp2p_traffic(client_host)
    try:
        yield
    finally:
        client_host.run(f"sudo -n sh {INJECT} unfreeze-runtime")
        for target in ("desired", "live", "lkg"):
            client_host.run(f"sudo -n sh {INJECT} unprotect {target}")
        cleanup_3xui(aux_host)


def test_runtime_apply_failure_preserves_external_subscription_state(
    client_host, aux_host, xp2p_client_runner, running_subscription
):
    baseline = _state_hashes(client_host)
    pid_before = _xray_pid(client_host)
    rotated = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh rotate-credentials")
    assert rotated.rc == 0, rotated.stderr
    frozen = client_host.run(f"sudo -n sh {INJECT} freeze-runtime")
    assert frozen.rc == 0, frozen.stderr
    try:
        failed = xp2p_client_runner(
            "client", "subscription", "refresh", "fixture", "--allow-http"
        )
        assert failed.rc != 0
        assert _state_hashes(client_host) == baseline
    finally:
        client_host.run(f"sudo -n sh {INJECT} unfreeze-runtime")
    assert _xray_pid(client_host) == pid_before


@pytest.mark.parametrize("target", ["desired", "live", "lkg"])
def test_persistence_failure_rolls_back_external_subscription(
    target, client_host, aux_host, xp2p_client_runner, running_subscription
):
    try:
        baseline = _state_hashes(client_host)
        rotated = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh rotate-credentials")
        assert rotated.rc == 0, rotated.stderr
        protected = client_host.run(f"sudo -n sh {INJECT} protect {target}")
        assert protected.rc == 0, protected.stderr
        failed = xp2p_client_runner(
            "client", "subscription", "refresh", "fixture", "--allow-http"
        )
        assert failed.rc != 0
        assert _state_hashes(client_host) == baseline
        marker = client_host.run("sudo -n test -s /etc/xp2p/.state/apply.error")
        assert marker.rc == 0, marker.stderr
    except AssertionError as error:
        pytest.fail(f"{error}\n{_failure_dump(client_host, aux_host)}")
    finally:
        client_host.run(f"sudo -n sh {INJECT} unprotect {target}")
    listener = client_host.run("sudo -n test -r /proc/$(pgrep -x xray | head -n1)/status")
    assert listener.rc == 0, listener.stderr


def test_concurrent_desired_edit_wins_over_external_refresh(
    client_host, aux_host, running_subscription
):
    baseline = _state_hashes(client_host)
    rotated = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh rotate-credentials")
    assert rotated.rc == 0, rotated.stderr
    conflict = client_host.run(f"sudo -n sh {INJECT} concurrent-refresh")
    assert conflict.rc == 0, conflict.stderr
    edit = client_host.run("sudo -n grep -Fq '# concurrent-user-edit' /etc/xp2p/xp2p-client.toml")
    assert edit.rc == 0, edit.stderr
    current = _state_hashes(client_host)
    assert current.splitlines()[1:] == baseline.splitlines()[1:]
    assert _xray_pid(client_host) > 0


def _xray_pid(host) -> int:
    result = host.run("pgrep -x xray | head -n1")
    assert result.rc == 0, result.stderr
    return int(result.stdout.strip())
