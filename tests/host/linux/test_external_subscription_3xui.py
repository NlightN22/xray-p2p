from __future__ import annotations

import pytest

from tests.host.linux._external_subscription_3xui import (
    CLIENT_DESIRED,
    CLIENT_LIVE,
    CLIENT_SUBSCRIPTION_LKG,
    CONTAINER,
    EXTENDED_MATRIX,
    FIXTURE_DIR,
    REMOTE_SUBSCRIPTION_URL,
    _assert_file_contains,
    _assert_live_protocol,
    _assert_live_security,
    _assert_offer_count,
    _failure_dump,
    _fetch_subscription,
    _fetch_subscription_headers,
    _state_hashes,
    _wait_for_xp2p_traffic,
    cleanup_3xui,
    setup_3xui,
)


@pytest.fixture(scope="module")
def pinned_3xui(aux_host):
    setup_3xui(aux_host)
    try:
        yield
    finally:
        cleanup_3xui(aux_host)


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

    headers = _fetch_subscription_headers(aux_host)
    assert headers.get("content-type", "").lower().startswith("text/plain")
    assert headers.get("profile-update-interval") == "12"
    assert headers.get("profile-web-page-url")
    assert headers.get("routing-enable") == "true"
    assert headers.get("subscription-userinfo")


def test_3xui_basic_import_uses_xp2p_live(
    client_host, xp2p_client_runner, pinned_3xui
):
    added = xp2p_client_runner(
        "client", "subscription", "add", "fixture", REMOTE_SUBSCRIPTION_URL, "--allow-http"
    )
    assert added.rc == 0, added.stderr
    started = xp2p_client_runner("client", "service", "start")
    assert started.rc == 0, started.stderr
    _assert_offer_count(xp2p_client_runner, 2)
    _assert_live_protocol(client_host, "trojan", present=True)
    _wait_for_xp2p_traffic(client_host)


@EXTENDED_MATRIX
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

        changed_security = aux_host.run(
            f"sudo -n sh {FIXTURE_DIR}/mutate.sh change-trojan-security"
        )
        assert changed_security.rc == 0, changed_security.stderr
        refreshed = xp2p_client_runner(
            "client", "subscription", "refresh", "fixture", "--allow-http"
        )
        assert refreshed.rc == 0, refreshed.stderr
        _assert_file_contains(client_host, CLIENT_DESIRED, 'alpn = ["h2", "http/1.1"]')
        _assert_file_contains(client_host, CLIENT_SUBSCRIPTION_LKG, '"alpn": [')
        _assert_live_security(client_host, "tls")
        _wait_for_xp2p_traffic(client_host)

        disabled = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh disable-trojan")
        assert disabled.rc == 0, disabled.stderr
        refreshed = xp2p_client_runner("client", "subscription", "refresh", "fixture", "--allow-http")
        assert refreshed.rc == 0, refreshed.stderr
        _assert_offer_count(xp2p_client_runner, 1)
        _assert_live_protocol(client_host, "trojan", present=False)
        _wait_for_xp2p_traffic(client_host)

        enabled = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh enable-trojan")
        assert enabled.rc == 0, enabled.stderr
        refreshed = xp2p_client_runner("client", "subscription", "refresh", "fixture", "--allow-http")
        assert refreshed.rc == 0, refreshed.stderr
        _assert_offer_count(xp2p_client_runner, 2)

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

    except AssertionError as error:
        pytest.fail(f"{error}\n{_failure_dump(client_host, aux_host)}")
