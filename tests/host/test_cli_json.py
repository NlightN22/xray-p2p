from __future__ import annotations

import json

import pytest

from tests.host import cli_json


def _envelope(result: dict) -> str:
    return json.dumps({"schema_version": "1", "command": "xp2p test", "result": result})


def test_credential_reads_install_and_user_add_results() -> None:
    expected = {"user": "alice", "password": "secret", "link": "trojan://secret@example#alice"}
    assert cli_json.credential(_envelope({"credential": expected})) == expected
    assert cli_json.credential(_envelope(expected)) == expected


@pytest.mark.parametrize(
    "result",
    [
        {"link": "trojan://one"},
        {"credential": {"link": "trojan://two"}},
        {"links": ["trojan://three"]},
        {"users": [{"link": "trojan://four"}]},
    ],
)
def test_link_reads_typed_link_results(result: dict) -> None:
    assert cli_json.link(_envelope(result)).startswith("trojan://")


def test_result_rejects_non_object_payload() -> None:
    with pytest.raises(AssertionError):
        cli_json.result(_envelope({"nested": []}).replace('{"nested": []}', "[]"))
