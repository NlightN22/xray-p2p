from __future__ import annotations


def parse_redirect_output(text: str) -> list[dict[str, str]]:
    lines = [line.strip() for line in (text or "").splitlines() if line.strip()]
    header_idx = None
    legacy = False
    for idx, line in enumerate(lines):
        lowered = line.lower()
        if (
            lowered.startswith("no redirect rules")
            or lowered.startswith("no server redirect rules")
            or lowered.startswith("no client redirect rules")
        ):
            return []
        if lowered.startswith("type"):
            header_idx = idx
            break
        if lowered.startswith("cidr"):
            legacy = True
            header_idx = idx
            break
    if header_idx is None:
        raise AssertionError(f"Unexpected redirect output: {text!r}")

    entries: list[dict[str, str]] = []
    for row in lines[header_idx + 1 :]:
        parts = row.split()
        if legacy:
            if len(parts) < 3:
                continue
            entries.append({"type": "CIDR", "value": parts[0], "cidr": parts[0], "tag": parts[1], "host": parts[2]})
            continue
        if len(parts) < 4:
            continue
        entry = {
            "type": parts[0],
            "value": parts[1],
            "tag": parts[2],
            "host": parts[3],
        }
        if entry["type"].lower() == "cidr":
            entry["cidr"] = entry["value"]
        entries.append(entry)
    return entries


def has_redirect_entry(entries: list[dict[str, str]], *, cidr: str, tag: str) -> bool:
    for entry in entries:
        if entry.get("cidr") == cidr and entry.get("tag") == tag:
            return True
    return False

