from __future__ import annotations

from urllib import parse


def extract_marker(output: str | None, marker: str) -> str | None:
    for raw in (output or "").splitlines():
        line = raw.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None


def assert_link_install_dir(link: str, expected_install_dir: str | None) -> None:
    parsed = parse.urlparse(link)
    query = parse.parse_qs(parsed.query)
    if expected_install_dir is None:
        assert "install_dir" not in query, f"install_dir should be omitted in link: {query}"
        return
    assert query.get("install_dir") == [expected_install_dir], f"install_dir mismatch: {query}"
