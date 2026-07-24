from __future__ import annotations

import json
from typing import Any


def result(output: str) -> dict[str, Any]:
    document = json.loads(output or "")
    value = document.get("result")
    if not isinstance(value, dict):
        raise AssertionError(f"xp2p JSON result is not an object: {value!r}")
    return value


def link(output: str) -> str:
    value = result(output)
    candidates: list[Any] = [value.get("link")]
    credential_value = value.get("credential")
    if isinstance(credential_value, dict):
        candidates.append(credential_value.get("link"))
    links = value.get("links")
    if isinstance(links, list):
        candidates.extend(links)
    users = value.get("users")
    if isinstance(users, list):
        candidates.extend(item.get("link") for item in users if isinstance(item, dict))
    for candidate in candidates:
        if isinstance(candidate, str) and candidate:
            return candidate
    raise AssertionError("xp2p JSON result does not contain a link")


def credential(output: str) -> dict[str, str]:
    value = result(output)
    nested = value.get("credential")
    if isinstance(nested, dict):
        value = nested
    user = value.get("user") or value.get("user_id")
    password = value.get("password")
    generated_link = value.get("link")
    if not isinstance(user, str) or not user:
        raise AssertionError("xp2p JSON credential does not contain a user")
    if not isinstance(password, str) or not password:
        raise AssertionError("xp2p JSON credential does not contain a password")
    if not isinstance(generated_link, str) or not generated_link:
        raise AssertionError("xp2p JSON credential does not contain a link")
    return {"user": user, "password": password, "link": generated_link}
