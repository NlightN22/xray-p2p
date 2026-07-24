from __future__ import annotations

import json

def parse_redirect_output(text: str) -> list[dict]:
    document = json.loads(text)
    if document.get("schema_version") != "1":
        raise AssertionError(f"Unexpected redirect contract: {document!r}")
    redirects = document.get("result", {}).get("redirects")
    if not isinstance(redirects, list):
        raise AssertionError(f"Redirect result is not a list: {document!r}")
    return [
        {
            **entry,
            "tag": entry.get("outbound_tag"),
            **({"cidr": entry.get("value")} if str(entry.get("type", "")).lower() == "cidr" else {}),
        }
        for entry in redirects
        if isinstance(entry, dict)
    ]


def has_redirect_entry(entries: list[dict[str, str]], *, cidr: str, tag: str) -> bool:
    for entry in entries:
        if entry.get("cidr") == cidr and entry.get("tag") == tag:
            return True
    return False

