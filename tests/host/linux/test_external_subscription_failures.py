import time
from pathlib import PurePosixPath

from tests.host.linux._external_subscription_3xui import EXTENDED_MATRIX


pytestmark = EXTENDED_MATRIX


CLIENT_DESIRED = PurePosixPath("/etc/xp2p/xp2p-client.toml")
CLIENT_LIVE = PurePosixPath("/etc/xp2p/.state/live/config-client/xray.json")
NEGATIVE_LKG = PurePosixPath("/etc/xp2p/.state/subscriptions/negative.json")
SOURCE_FIXTURE = PurePosixPath(
    "/srv/xray-p2p/tests/guest/scripts/linux/subscription_source_fixture.sh"
)


def test_external_subscription_failures_preserve_applied_state(
    client_host, xp2p_client_runner
):
    started = client_host.run(f"sudo -n sh {SOURCE_FIXTURE} start")
    assert started.rc == 0, started.stderr
    try:
        added = xp2p_client_runner(
            "client",
            "subscription",
            "add",
            "negative",
            "http://127.0.0.1:18096/subscription.txt",
            "--allow-http",
        )
        assert added.rc == 0, added.stderr
        service = xp2p_client_runner("client", "service", "start")
        assert service.rc == 0, service.stderr
        _assert_live_protocol(client_host, "trojan")
        baseline = _state_hashes(client_host)

        for mode in ("malformed", "unsupported", "oversized"):
            changed = client_host.run(f"sudo -n sh {SOURCE_FIXTURE} set {mode}")
            assert changed.rc == 0, changed.stderr
            failed = xp2p_client_runner(
                "client", "subscription", "refresh", "negative", "--allow-http"
            )
            assert failed.rc != 0
            assert "fixture-negative-secret" not in failed.stderr
            assert _state_hashes(client_host) == baseline
            leaked = client_host.run(
                "sudo -n grep -R -Fq 'fixture-negative-secret' "
                "/var/log/xp2p /etc/xp2p/audit.log"
            )
            assert leaked.rc != 0
    finally:
        client_host.run(f"sudo -n sh {SOURCE_FIXTURE} stop")


def test_external_subscription_refresh_stages_while_service_is_stopped(
    client_host, xp2p_client_runner
):
    started = client_host.run(f"sudo -n sh {SOURCE_FIXTURE} start")
    assert started.rc == 0, started.stderr
    try:
        added = xp2p_client_runner(
            "client",
            "subscription",
            "add",
            "staged",
            "http://127.0.0.1:18096/subscription.txt",
            "--allow-http",
        )
        assert added.rc == 0, added.stderr
        service = xp2p_client_runner("client", "service", "start")
        assert service.rc == 0, service.stderr
        _assert_file_contains(client_host, CLIENT_LIVE, "fixture-negative-secret")

        stopped = xp2p_client_runner("client", "service", "stop")
        assert stopped.rc == 0, stopped.stderr
        changed = client_host.run(f"sudo -n sh {SOURCE_FIXTURE} set rotated")
        assert changed.rc == 0, changed.stderr
        refreshed = xp2p_client_runner(
            "client", "subscription", "refresh", "staged", "--allow-http"
        )
        assert refreshed.rc == 0, refreshed.stderr
        _assert_file_contains(client_host, CLIENT_DESIRED, "fixture-rotated-secret")
        _assert_file_contains(client_host, CLIENT_LIVE, "fixture-negative-secret")

        restarted = xp2p_client_runner("client", "service", "start")
        assert restarted.rc == 0, restarted.stderr
        _assert_file_contains(client_host, CLIENT_LIVE, "fixture-rotated-secret")
    finally:
        client_host.run(f"sudo -n sh {SOURCE_FIXTURE} stop")


def _assert_live_protocol(host, protocol: str) -> None:
    deadline = time.time() + 30
    result = None
    while time.time() < deadline:
        result = host.run(
            f"sudo -n grep -Eq '\"protocol\"[[:space:]]*:[[:space:]]*\"{protocol}\"' "
            f"{CLIENT_LIVE}"
        )
        if result.rc == 0:
            return
        time.sleep(1)
    raise AssertionError(
        f"Live protocol {protocol} is absent; exit {result.rc if result else 'unknown'}"
    )


def _assert_file_contains(host, path: PurePosixPath, value: str) -> None:
    deadline = time.time() + 30
    result = None
    while time.time() < deadline:
        result = host.run(f"sudo -n grep -Fq -- '{value}' {path}")
        if result.rc == 0:
            return
        time.sleep(1)
    raise AssertionError(
        f"{path} does not contain expected value; exit {result.rc if result else 'unknown'}"
    )


def _state_hashes(host) -> str:
    result = host.run(
        f"sudo -n sha256sum {CLIENT_DESIRED} {CLIENT_LIVE}; "
        f"sudo -n sed -n '/\"revision\":/,/\"last_refresh_at\":/p' {NEGATIVE_LKG} "
        "| sha256sum"
    )
    assert result.rc == 0, result.stderr
    return result.stdout
