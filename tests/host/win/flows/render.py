from __future__ import annotations

import json
from typing import Any

import pytest


def extract_first_json(stdout: str, *, label: str) -> Any:
    raw = stdout or ""
    start = raw.find("{")
    if start < 0:
        pytest.fail(f"Expected JSON object in output for {label}.\nSTDOUT:\n{stdout}")
    payload = raw[start:]
    try:
        dec = json.JSONDecoder()
        value, _end = dec.raw_decode(payload)
        return value
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON output for {label}: {exc}\nSTDOUT:\n{stdout}")


def render_desired_xray_json(runner, *, role: str, config_path: str | None = None) -> dict:
    args: list[str] = []
    if config_path:
        args.extend(["--config", str(config_path)])
    args.extend([role, "render", "xray", "--desired", "--output", "-"])
    result = runner(*args, check=True)
    value = extract_first_json(result.stdout or "", label=f"xp2p {role} render xray --desired")
    if not isinstance(value, dict):
        pytest.fail(f"Unexpected JSON type from xp2p {role} render: {type(value).__name__}")
    return value
