from __future__ import annotations

import base64
from pathlib import PurePosixPath

import pytest

FIXTURE_DIR = PurePosixPath("/srv/xray-p2p/infra/vagrant/debian12/deb-test/3x-ui")
CONTAINER = "xp2p-3x-ui-v2-8-11"
SUBSCRIPTION_URL = "http://127.0.0.1:2096/sub/xp2pfixture2811"
REMOTE_SUBSCRIPTION_URL = "http://10.62.10.13:2096/sub/xp2pfixture2811"


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

    response = aux_host.run(f"curl --fail --silent {SUBSCRIPTION_URL}")
    assert response.rc == 0, response.stderr
    decoded = _decode_subscription(response.stdout)
    assert len(decoded) == 2
    assert any(item.startswith("trojan://fixture-trojan-password@") for item in decoded)
    assert any(item.startswith("vless://550e8400-e29b-41d4-a716-446655440000@") for item in decoded)


def test_xp2p_imports_real_3xui_subscription(xp2p_client_runner, pinned_3xui):
    added = xp2p_client_runner(
        "client", "subscription", "add", "fixture", REMOTE_SUBSCRIPTION_URL, "--allow-http"
    )
    assert added.rc == 0, added.stderr

    status = xp2p_client_runner("client", "subscription", "status")
    assert status.rc == 0, status.stderr
    assert "ID: fixture" in status.stdout
    assert "Offers: 2" in status.stdout

    offers = xp2p_client_runner("client", "subscription", "offers")
    assert offers.rc == 0, offers.stderr
    assert offers.stdout.count("fixture\toffer-") == 2
    assert "\ttrojan\t" in offers.stdout
    assert "\tvless\t" in offers.stdout


def test_3xui_trojan_and_vless_offers_carry_real_traffic(client_host, pinned_3xui):
    trojan = client_host.run(
        "sudo -n sh /srv/xray-p2p/tests/guest/scripts/linux/check_3xui_offer_traffic.sh "
        "trojan fixture-trojan-password 16443 19081"
    )
    assert trojan.rc == 0, trojan.stderr

    vless = client_host.run(
        "sudo -n sh /srv/xray-p2p/tests/guest/scripts/linux/check_3xui_offer_traffic.sh "
        "vless 550e8400-e29b-41d4-a716-446655440000 16444 19082"
    )
    assert vless.rc == 0, vless.stderr


def test_3xui_refresh_tracks_credentials_removal_and_restart(aux_host, pinned_3xui):
    rotated = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh rotate-credentials")
    assert rotated.rc == 0, rotated.stderr
    decoded = _fetch_subscription(aux_host)
    assert any(item.startswith("trojan://rotated-trojan-password@") for item in decoded)
    assert any(item.startswith("vless://8b1a9953-c461-4c0f-8c8f-7e6f40c6f0ad@") for item in decoded)

    removed = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/mutate.sh remove-vless")
    assert removed.rc == 0, removed.stderr
    decoded = _fetch_subscription(aux_host)
    assert len(decoded) == 1
    assert decoded[0].startswith("trojan://rotated-trojan-password@")

    restarted = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose restart")
    assert restarted.rc == 0, restarted.stderr
    decoded = _fetch_subscription(aux_host)
    assert len(decoded) == 1
    assert decoded[0].startswith("trojan://rotated-trojan-password@")


def _decode_subscription(value: str) -> list[str]:
    raw = value.strip()
    if "://" not in raw:
        raw = base64.b64decode(raw).decode("utf-8")
    return [line.strip() for line in raw.splitlines() if line.strip()]


def _fetch_subscription(host) -> list[str]:
    response = host.run(
        f"curl --fail --silent --retry 20 --retry-delay 1 --retry-connrefused {SUBSCRIPTION_URL}"
    )
    assert response.rc == 0, response.stderr
    return _decode_subscription(response.stdout)
