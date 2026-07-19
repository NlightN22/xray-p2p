from __future__ import annotations

import json
import os
import re
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _apply_request
from tests.host.linux import env as linux_env

try:
    import tomllib
except ImportError:  # pragma: no cover - fallback for older runtimes.
    import tomli as tomllib


SKIP_MANUAL_EDIT = os.environ.get("XP2P_RUN_MANUAL_EDIT_TESTS", "").strip().lower() not in {"1", "true", "yes"}
pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.skipif(SKIP_MANUAL_EDIT, reason="manual edit tests are opt-in")]

APPLY_REQUEST = helpers.STATE_ROOT / "apply.request"
APPLY_ERROR = helpers.STATE_ROOT / "apply.error"
DESIRED_CLIENT_CONFIG = helpers.CLIENT_CONFIG_FILE
DESIRED_CLIENT_EXT_DIR = helpers.CLIENT_CONFIG_DIR
DESIRED_CLIENT_OUTBOUNDS_APPEND = DESIRED_CLIENT_EXT_DIR / "outbounds.append.json"
LIVE_CLIENT_META = helpers.CLIENT_LIVE_DIR / "runtime.json"
LIVE_CLIENT_XRAY = helpers.CLIENT_LIVE_DIR / "xray.json"
LKG_CLIENT_META = helpers.CONFIG_LKG_ROOT / helpers.CLIENT_CONFIG_DIR_NAME / "runtime.json"

POLL_INTERVAL = 1.5
TIMEOUT = 60.0


def _wait_for_path(host, path, *, exists: bool, timeout: float = TIMEOUT) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        present = linux_env.path_exists(host, path)
        if present == exists:
            return
        time.sleep(POLL_INTERVAL)
    state = "present" if exists else "absent"
    raise AssertionError(f"Expected {path} to be {state} after {timeout} seconds.")


def _read_toml(host, path):
    content = linux_env.read_text(host, path)
    try:
        return tomllib.loads(content)
    except tomllib.TOMLDecodeError as exc:
        raise AssertionError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


def _read_json(host, path) -> dict:
    raw = linux_env.read_text(host, path)
    try:
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        raise AssertionError(f"Failed to parse JSON from {path}: {exc}\nContent:\n{raw}") from exc


def _write_json(host, path, payload: dict) -> None:
    linux_env.write_text(host, path, json.dumps(payload, indent=2, sort_keys=True) + "\n")


def _set_int_setting(content: str, key: str, value: int) -> str:
    want = re.sub(r"[^a-z0-9]", "", key.lower())
    lines = content.splitlines(keepends=True)
    for idx, line in enumerate(lines):
        if "=" not in line:
            continue
        left, right = line.split("=", 1)
        got = re.sub(r"[^a-z0-9]", "", left.lower())
        if got != want and not got.endswith(want):
            continue
        match = re.match(r"(?P<ws>\s*)(?P<num>\d+)(?P<tail>.*)\Z", right, re.DOTALL)
        if not match:
            continue
        lines[idx] = f"{left}={match.group('ws')}{int(value)}{match.group('tail')}"
        return "".join(lines)
    candidates = []
    for line in lines:
        if "mtu" not in line.lower():
            continue
        candidates.append((line.rstrip("\n"), [hex(ord(ch)) for ch in line.rstrip("\n")]))
    raise AssertionError(
        f"Expected {key} to be present in config:\\n{content}\\n"
        f"Candidate lines (repr + codepoints):\\n{candidates}"
    )


def _read_apply_request(host) -> dict:
    raw = linux_env.read_text(host, APPLY_REQUEST)
    try:
        document = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise AssertionError(f"Failed to parse apply.request JSON: {exc}\nContent:\n{raw}") from exc
    return _apply_request.role_request(document, "client")


def _read_apply_error(host) -> dict:
    raw = linux_env.read_text(host, APPLY_ERROR)
    try:
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        raise AssertionError(f"Failed to parse apply.error JSON: {exc}\nContent:\n{raw}") from exc


def _read_runtime_meta(host, path) -> dict:
    raw = linux_env.read_text(host, path)
    try:
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        raise AssertionError(f"Failed to parse runtime meta JSON from {path}: {exc}\nContent:\n{raw}") from exc


def _compiled_at(meta: dict) -> str:
    value = meta.get("compiled_at")
    if isinstance(value, str) and value.strip():
        return value.strip()
    return ""


def _install_client(runner) -> None:
    runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--host",
        "10.55.20.10",
        "--user",
        "manual-edit@example.com",
        "--password",
        "manual-edit-secret",
        "--force",
        check=True,
    )


def _start_client_service(runner) -> None:
    runner("client", "service", "stop")
    runner("client", "service", "start", check=True)


def _stop_client_service(runner) -> None:
    runner("client", "service", "stop")


@pytest.mark.host
@pytest.mark.linux
def test_manual_edit_applies_pending_snapshot(client_host, xp2p_client_runner):
    try:
        _install_client(xp2p_client_runner)
        _start_client_service(xp2p_client_runner)

        _wait_for_path(client_host, APPLY_REQUEST, exists=False, timeout=TIMEOUT)
        _wait_for_path(client_host, LIVE_CLIENT_META, exists=True, timeout=TIMEOUT)

        extra_tag = "user-manual-extra"
        outbounds_append = {
            "outbounds": [
                {"protocol": "freedom", "settings": {}, "tag": extra_tag},
            ]
        }
        linux_env.write_text(
            client_host,
            DESIRED_CLIENT_OUTBOUNDS_APPEND,
            json.dumps(outbounds_append, indent=2, sort_keys=True) + "\n",
        )

        baseline_meta = _read_runtime_meta(client_host, LIVE_CLIENT_META)
        baseline_compiled_at = _compiled_at(baseline_meta)
        original = linux_env.read_text(client_host, DESIRED_CLIENT_CONFIG)
        updated = _set_int_setting(original, "tun_mtu", 1400)
        linux_env.write_text(client_host, DESIRED_CLIENT_CONFIG, updated)

        _wait_for_path(client_host, APPLY_REQUEST, exists=False, timeout=TIMEOUT)
        meta = _read_runtime_meta(client_host, LIVE_CLIENT_META)
        assert meta.get("tun_mtu") == 1400
        assert _compiled_at(meta) != baseline_compiled_at or baseline_compiled_at == ""
        xray = _read_json(client_host, LIVE_CLIENT_XRAY)
        outbounds = xray.get("outbounds") or []
        assert any(isinstance(item, dict) and item.get("tag") == extra_tag for item in outbounds)
        assert not linux_env.path_exists(client_host, APPLY_ERROR)
    except Exception:
        helpers.dump_failure_state(client_host, "manual-edit-apply")
        raise
    finally:
        _stop_client_service(xp2p_client_runner)
        helpers.cleanup_client_install(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_manual_edit_rollback_on_invalid_pending(client_host, xp2p_client_runner):
    try:
        _install_client(xp2p_client_runner)
        _start_client_service(xp2p_client_runner)

        _wait_for_path(client_host, APPLY_REQUEST, exists=False, timeout=TIMEOUT)
        _wait_for_path(client_host, LIVE_CLIENT_META, exists=True, timeout=TIMEOUT)
        baseline_live_hash = linux_env.file_sha256(client_host, LIVE_CLIENT_META)

        original_config = linux_env.read_text(client_host, DESIRED_CLIENT_CONFIG)
        linux_env.write_text(client_host, DESIRED_CLIENT_CONFIG, "this is not toml\n")

        _wait_for_path(client_host, APPLY_REQUEST, exists=True, timeout=TIMEOUT)
        request = _read_apply_request(client_host)
        _wait_for_path(client_host, APPLY_ERROR, exists=True, timeout=TIMEOUT * 2)
        error = _read_apply_error(client_host)

        assert linux_env.path_exists(client_host, APPLY_REQUEST)
        assert error.get("request_id") == request.get("id")
        assert (error.get("reason") or "").strip(), "apply.error missing reason"

        live_hash = linux_env.file_sha256(client_host, LIVE_CLIENT_META)
        assert live_hash == baseline_live_hash

        linux_env.write_text(client_host, DESIRED_CLIENT_CONFIG, original_config)
        _wait_for_path(client_host, APPLY_ERROR, exists=False, timeout=TIMEOUT * 2)
        _wait_for_path(client_host, APPLY_REQUEST, exists=False, timeout=TIMEOUT * 2)
        _wait_for_path(client_host, LIVE_CLIENT_META, exists=True, timeout=TIMEOUT * 2)
    except Exception:
        helpers.dump_failure_state(client_host, "manual-edit-rollback")
        raise
    finally:
        _stop_client_service(xp2p_client_runner)
        helpers.cleanup_client_install(client_host, xp2p_client_runner)
