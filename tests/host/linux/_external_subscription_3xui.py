import base64
import os
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
FAILURE_DUMP = PurePosixPath(
    "/srv/xray-p2p/tests/guest/scripts/linux/dump_external_subscription_state.sh"
)
RUN_EXTENDED_MATRIX = os.environ.get(
    "XP2P_RUN_EXTERNAL_SUBSCRIPTION_MATRIX", ""
).strip().lower() in {"1", "true", "yes"}
EXTENDED_MATRIX = pytest.mark.skipif(
    not RUN_EXTENDED_MATRIX,
    reason="set XP2P_RUN_EXTERNAL_SUBSCRIPTION_MATRIX=1 to run the extended external subscription matrix",
)


def _assert_offer_count(runner, expected: int) -> None:
    status = runner("client", "subscription", "status")
    assert status.rc == 0, status.stderr
    assert f"Offers: {expected}" in status.stdout


def setup_3xui(aux_host) -> None:
    reset = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/reset.sh")
    assert reset.rc == 0, reset.stderr
    started = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose up -d")
    assert started.rc == 0, started.stderr
    setup = aux_host.run(f"sudo -n sh {FIXTURE_DIR}/setup.sh")
    assert setup.rc == 0, setup.stderr


def cleanup_3xui(aux_host) -> None:
    aux_host.run(f"sudo -n sh {FIXTURE_DIR}/snapshot-override.sh stop")
    aux_host.run(f"sudo -n sh {FIXTURE_DIR}/reset.sh")


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


def _wait_for_file_contains(host, path: PurePosixPath, value: str) -> None:
    deadline = time.time() + 30
    result = None
    while time.time() < deadline:
        result = host.run(f"sudo -n grep -Fq -- '{value}' {path}")
        if result.rc == 0:
            return
        time.sleep(1)
    raise AssertionError(result.stderr if result else f"Timed out waiting for {path}")


def _assert_live_security(host, security: str) -> None:
    result = host.run(
        f"sudo -n grep -Eq '\"security\"[[:space:]]*:[[:space:]]*\"{security}\"' {CLIENT_LIVE}"
    )
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
    state = client_host.run(f"sudo -n sh {FAILURE_DUMP}")
    services = client_host.run("sudo -n systemctl --no-pager --full status xp2p-client.service")
    panel = aux_host.run(f"cd {FIXTURE_DIR} && sudo -n docker-compose ps")
    return (
        "Sanitized failure dump:\n"
        f"subscription state:\n{state.stdout}\n"
        f"client service:\n{services.stdout[-4000:]}\n"
        f"3x-ui containers:\n{panel.stdout}"
    )
