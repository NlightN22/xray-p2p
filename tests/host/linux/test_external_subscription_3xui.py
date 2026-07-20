from __future__ import annotations

import base64
from pathlib import PurePosixPath

import pytest

FIXTURE_DIR = PurePosixPath("/srv/xray-p2p/infra/vagrant/debian12/deb-test/3x-ui")
CONTAINER = "xp2p-3x-ui-v2-8-11"
SUBSCRIPTION_URL = "http://127.0.0.1:2096/sub/xp2pfixture2811"


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


def _decode_subscription(value: str) -> list[str]:
    raw = value.strip()
    if "://" not in raw:
        raw = base64.b64decode(raw).decode("utf-8")
    return [line.strip() for line in raw.splitlines() if line.strip()]
