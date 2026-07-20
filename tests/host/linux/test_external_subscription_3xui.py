from __future__ import annotations

import base64
import time
from pathlib import PurePosixPath

import pytest

FIXTURE_DIR = PurePosixPath("/srv/xray-p2p/infra/vagrant/debian12/deb-test/3x-ui")
CONTAINER = "xp2p-3x-ui-v2-8-11"
SUBSCRIPTION_URL = "http://127.0.0.1:2096/sub/xp2pfixture2811"
REMOTE_SUBSCRIPTION_URL = "http://10.62.10.13:2096/sub/xp2pfixture2811"
CLIENT_LIVE = PurePosixPath("/etc/xp2p/.state/live/config-client/xray.json")
CLIENT_DESIRED = PurePosixPath("/etc/xp2p/xp2p-client.toml")
CLIENT_SUBSCRIPTION_LKG = PurePosixPath("/etc/xp2p/.state/subscriptions/fixture.json")


@pytest.fixture(scope="module")
def pinned_3xui(aux_host):
    reset = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/reset.sh")
    assert reset.rc == 0, reset.stderr
    started = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose up -d")
    assert started.rc == 0, started.stderr
    setup = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/setup.sh")
    assert setup.rc == 0, setup.stderr
    try:
        yield
    finally:
        aux_host.run(f"sudo -n sh {FIXTURE_DIR}/reset.sh")


def test_3xui_pinned_versions_and_subscription_contract(aux_host, pinned_3xui):
    panel = aux_host.run(f"sudo -n docker exec {CONTAINER} /app/x-ui -v")
    assert panel.rc == 0, panel.stderr
    assert "2.8.11" in panel.stdout

    xray = aux_host.run(f"sudo -n docker exec {CONTAINER} /app/bin/xray-linux-amd64 version")
    assert xray.rc == 0, xray.stderr
    assert "26.2.6" in xray.stdout

    decoded = _fetch_subscription(aux_host)
    assert len(decoded) == 2
    assert any(item.startswith("trojan://fixture-trojan-password@") for item in decoded)
    assert any(item.startswith("vless://550e8400-e29b-41d4-a716-446655440000@") for item in decoded)


def test_3xui_offers_flow_through_xp2p_live(
    client_host, aux_host, xp2p_client_runner, pinned_3xui
):
    try:
        added = xp2p_client_runner(
            "client", "subscription", "add", "fixture", REMOTE_SUBSCRIPTION_URL, "--allow-http"
        )
        assert added.rc == 0, added.stderr
        _assert_offer_count(xp2p_client_runner, 2)

        started = xp2p_client_runner("client", "service", "start")
        assert started.rc == 0, started.stderr
        _assert_live_protocol(client_host, "trojan", present=True)
        _wait_for_xp2p_traffic(client_host)

        rotated = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh rotate-credentials")
        assert rotated.rc == 0, rotated.stderr
        refreshed = xp2p_client_runner("client", "subscription", "refresh", "fixture", "--allow-http")
        assert refreshed.rc == 0, refreshed.stderr
        _assert_offer_count(xp2p_client_runner, 2)
        _assert_file_contains(client_host, CLIENT_DESIRED, "rotated-trojan-password")
        _assert_file_contains(client_host, CLIENT_LIVE, "rotated-trojan-password")
        _assert_file_contains(client_host, CLIENT_SUBSCRIPTION_LKG, "rotated-trojan-password")
        _wait_for_xp2p_traffic(client_host)

        removed = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh remove-trojan")
        assert removed.rc == 0, removed.stderr
        refreshed = xp2p_client_runner("client", "subscription", "refresh", "fixture", "--allow-http")
        assert refreshed.rc == 0, refreshed.stderr
        _assert_offer_count(xp2p_client_runner, 1)
        _assert_live_protocol(client_host, "trojan", present=False)
        _assert_live_protocol(client_host, "vless", present=True)
        _wait_for_xp2p_traffic(client_host)

        before_failure = _state_hashes(client_host)
        stopped = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose stop")
        assert stopped.rc == 0, stopped.stderr
        failed = xp2p_client_runner("client", "subscription", "refresh", "fixture", "--allow-http")
        assert failed.rc != 0
        assert _state_hashes(client_host) == before_failure

        restarted = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose start")
        assert restarted.rc == 0, restarted.stderr
        _fetch_subscription(aux_host)
        client_restarted = xp2p_client_runner("client", "service", "restart")
        assert client_restarted.rc == 0, client_restarted.stderr
        refreshed = xp2p_client_runner("client", "subscription", "refresh", "fixture", "--allow-http")
        assert refreshed.rc == 0, refreshed.stderr
        _wait_for_xp2p_traffic(client_host)
    except AssertionError as error:
        pytest.fail(f"{error}\n{_failure_dump(client_host, aux_host)}")


def _assert_offer_count(runner, expected: int) -> None:
    status = runner("client", "subscription", "status")
    assert status.rc == 0, status.stderr
    assert f"Offers: {expected}" in status.stdout


def _wait_for_xp2p_traffic(host) -> None:
    deadline = time.time() + 30
    last = None
    while time.time() < deadline:
        last = host.run(
            "curl --fail --silent --max-time 5 --socks5-hostname 127.0.0.1:51180 "
            "--output /dev/null https://example.com/"
        )
        if last.rc == 0:
            return
        time.sleep(1)
    raise AssertionError(f"XP2P SOCKS traffic failed with exit {last.rc if last else 'unknown'}")


def _assert_live_protocol(host, protocol: str, present: bool) -> None:
    deadline = time.time() + 30
    result = None
    while time.time() < deadline:
        ready = host.run(f"sudo -n test -s {CLIENT_LIVE}")
        if ready.rc != 0:
            time.sleep(1)
            continue
        result = host.run(
            f"sudo -n grep -Eq '\"protocol\"[[:space:]]*:[[:space:]]*\"{protocol}\"' {CLIENT_LIVE}"
        )
        if (result.rc == 0) == present:
            return
        time.sleep(1)
    raise AssertionError(
        f"Live protocol {protocol} presence is not {present}; exit {result.rc if result else 'unknown'}"
    )


def _assert_file_contains(host, path: PurePosixPath, value: str) -> None:
    result = host.run(f"sudo -n grep -Fq -- '{value}' {path}")
    assert result.rc == 0, result.stderr


def _state_hashes(host) -> str:
    result = host.run(
        f"sudo -n sha256sum {CLIENT_DESIRED} {CLIENT_LIVE}; "
        f"sudo -n sed -n '/\"revision\":/,/\"last_refresh_at\":/p' {CLIENT_SUBSCRIPTION_LKG} "
        "| sha256sum"
    )
    assert result.rc == 0, result.stderr
    return result.stdout


def _fetch_subscription(host) -> list[str]:
    response = host.run(
        f"curl --fail --silent --retry 20 --retry-delay 1 --retry-connrefused {SUBSCRIPTION_URL}"
    )
    assert response.rc == 0, response.stderr
    raw = response.stdout.strip()
    if "://" not in raw:
        raw = base64.b64decode(raw).decode("utf-8")
    return [line.strip() for line in raw.splitlines() if line.strip()]


def _failure_dump(client_host, aux_host) -> str:
    tree = client_host.run("sudo -n find /etc/xp2p/.state -maxdepth 4 -printf '%y %p\\n'")
    services = client_host.run("sudo -n systemctl --no-pager --full status xp2p-client.service")
    panel = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose ps")
    return (
        "Sanitized failure dump:\n"
        f"state tree:\n{tree.stdout}\n"
        f"client service:\n{services.stdout[-4000:]}\n"
        f"3x-ui containers:\n{panel.stdout}"
    )
