from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import parsers

CLIENT_STATE_FILE = helpers.CLIENT_CONFIG_FILE
INSTALL_PATH = helpers.INSTALL_ROOT.as_posix()
CONFIG_DIR = helpers.CLIENT_CONFIG_DIR_NAME
PRIMARY_HOST = "10.240.0.10"
SECONDARY_HOST = "10.240.0.11"
REDIRECT_CIDR = "10.230.0.0/16"
REDIRECT_DOMAIN = "svc.internal.example"
INVALID_CIDR = "10.999.0.0/33"


def _install_endpoint(runner, host: str, user: str, password: str) -> None:
    runner(
        "client",
        "install",
        "--path",
        INSTALL_PATH,
        "--config-dir",
        CONFIG_DIR,
        "--host",
        host,
        "--user",
        user,
        "--password",
        password,
        check=True,
    )


def _redirect_cmd(runner, subcommand: str, *args: str, check: bool = False):
    base = [
        "client",
        "redirect",
        subcommand,
        "--path",
        INSTALL_PATH,
        "--config-dir",
        CONFIG_DIR,
    ]
    if subcommand in {"add", "remove"}:
        base.append("--quiet")
    if subcommand == "list":
        base.append("--pending")
    base.extend(args)
    return runner(*base, check=check)


def _list_redirects(runner):
    result = _redirect_cmd(runner, "list", check=True)
    return result.stdout or "", parsers.parse_redirect_output(result.stdout or "")


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".lower()


@pytest.mark.host
@pytest.mark.linux
def test_client_redirect_add_remove_and_cleanup(client_host, xp2p_client_runner):
    try:
        _install_endpoint(xp2p_client_runner, PRIMARY_HOST, "primary@example.com", "primary-pass")
        _install_endpoint(xp2p_client_runner, SECONDARY_HOST, "secondary@example.com", "secondary-pass")

        empty_output, entries = _list_redirects(xp2p_client_runner)
        assert REDIRECT_CIDR not in empty_output
        assert entries == [] or all(entry["value"] in {PRIMARY_HOST + "/32", SECONDARY_HOST + "/32"} for entry in entries)
        primary_tag = helpers.expected_proxy_tag(PRIMARY_HOST)
        secondary_tag = helpers.expected_proxy_tag(SECONDARY_HOST)

        _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            REDIRECT_CIDR,
            "--tag",
            primary_tag,
            check=True,
        )

        list_output, records = _list_redirects(xp2p_client_runner)
        assert REDIRECT_CIDR in list_output
        assert any(
            rec.get("cidr") == REDIRECT_CIDR and rec["tag"] == primary_tag and rec["type"].lower() == "cidr"
            for rec in records
        )

        routing = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_redirect_rule(routing, REDIRECT_CIDR, primary_tag)
        helpers.assert_routing_rule(routing, PRIMARY_HOST)
        helpers.assert_routing_rule(routing, SECONDARY_HOST)

        _redirect_cmd(
            xp2p_client_runner,
            "remove",
            "--cidr",
            REDIRECT_CIDR,
            "--host",
            PRIMARY_HOST,
            check=True,
        )

        routing_after_remove = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_no_redirect_rule(routing_after_remove, REDIRECT_CIDR)
        _, after_records = _list_redirects(xp2p_client_runner)
        assert all(rec.get("cidr") != REDIRECT_CIDR for rec in after_records)

        _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            REDIRECT_CIDR,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        list_output, records = _list_redirects(xp2p_client_runner)
        assert any(rec["host"] == SECONDARY_HOST for rec in records)
        routing = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_redirect_rule(routing, REDIRECT_CIDR, secondary_tag)

        _redirect_cmd(
            xp2p_client_runner,
            "remove",
            "--cidr",
            REDIRECT_CIDR,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        routing = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_no_redirect_rule(routing, REDIRECT_CIDR)

        invalid_tag_result = _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            INVALID_CIDR,
            "--tag",
            primary_tag,
            check=False,
        )
        assert invalid_tag_result.rc != 0
        assert "invalid cidr" in _combined_output(invalid_tag_result)

        invalid_host_result = _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            INVALID_CIDR,
            "--host",
            SECONDARY_HOST,
            check=False,
        )
        assert invalid_host_result.rc != 0
        assert "invalid cidr" in _combined_output(invalid_host_result)

        _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--domain",
            REDIRECT_DOMAIN,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        routing = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_domain_redirect_rule(routing, REDIRECT_DOMAIN, secondary_tag)
        list_output, records = _list_redirects(xp2p_client_runner)
        assert any(
            rec["type"].lower() == "domain"
            and rec["value"].lower() == REDIRECT_DOMAIN
            and rec["host"] == SECONDARY_HOST
            for rec in records
        )

        _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            REDIRECT_CIDR,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        routing = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_redirect_rule(routing, REDIRECT_CIDR, secondary_tag)
        helpers.assert_domain_redirect_rule(routing, REDIRECT_DOMAIN, secondary_tag)
        list_output, records = _list_redirects(xp2p_client_runner)
        assert any(rec.get("cidr") == REDIRECT_CIDR for rec in records)
        assert any(rec["value"].lower() == REDIRECT_DOMAIN for rec in records)

        _redirect_cmd(
            xp2p_client_runner,
            "remove",
            "--domain",
            REDIRECT_DOMAIN,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        routing = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_no_domain_redirect_rule(routing, REDIRECT_DOMAIN, secondary_tag)
        helpers.assert_redirect_rule(routing, REDIRECT_CIDR, secondary_tag)

        xp2p_client_runner(
            "client",
            "remove",
            "--path",
            INSTALL_PATH,
            "--config-dir",
            CONFIG_DIR,
            "--quiet",
            SECONDARY_HOST,
            check=True,
        )

        auto_output, auto_records = _list_redirects(xp2p_client_runner)
        assert REDIRECT_CIDR not in auto_output
        assert all(rec["value"] == f"{PRIMARY_HOST}/32" for rec in auto_records)

        routing = helpers.render_xray(client_host, xp2p_client_runner, "client", desired=True)
        helpers.assert_no_redirect_rule(routing, REDIRECT_CIDR)
        helpers.assert_no_domain_redirect_rule(routing, REDIRECT_DOMAIN)
        helpers.assert_routing_rule(routing, PRIMARY_HOST)

        state = helpers.read_pending_client_config(client_host)
        redirects = state.get("redirects") or []
        assert redirects == []
        remaining_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
        assert remaining_hosts == {PRIMARY_HOST}

        missing_remove = xp2p_client_runner(
            "client",
            "remove",
            "--path",
            INSTALL_PATH,
            "--config-dir",
            CONFIG_DIR,
            "--quiet",
            SECONDARY_HOST,
            check=False,
        )
        assert missing_remove.rc != 0
        assert f'client endpoint "{SECONDARY_HOST}" not found' in _combined_output(missing_remove)

        xp2p_client_runner(
            "client",
            "remove",
            "--path",
            INSTALL_PATH,
            "--config-dir",
            CONFIG_DIR,
            "--all",
            "--quiet",
            check=True,
        )
        _, records = _list_redirects(xp2p_client_runner)
        assert all(rec.get("cidr") != REDIRECT_CIDR for rec in records)
        assert records == []
    finally:
        pass
