from __future__ import annotations

from tests.host.linux import _helpers as helpers

INSTALL_PATH = helpers.INSTALL_ROOT.as_posix()
SERVER_HOST = "runtime-disable-server.example.com"
CLIENT_PRIMARY = "10.74.10.10"
CLIENT_SECONDARY = "10.74.10.11"
CLIENT_REDIRECT_CIDR = "10.74.20.0/24"
CLIENT_REDIRECT_DOMAIN = "runtime-disable.internal"


def install_server_with_users(runner) -> tuple[str, str]:
    runner(
        "server",
        "install",
        "--path",
        INSTALL_PATH,
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--host",
        SERVER_HOST,
        "--port",
        "62170",
        "--force",
        check=True,
    )
    runner(
        "server",
        "user",
        "add",
        "--path",
        INSTALL_PATH,
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--id",
        "runtime-alpha@example.com",
        "--password",
        "runtime-alpha-pass",
        "--host",
        SERVER_HOST,
        check=True,
    )
    runner(
        "server",
        "user",
        "add",
        "--path",
        INSTALL_PATH,
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--id",
        "runtime-bravo@example.com",
        "--password",
        "runtime-bravo-pass",
        "--host",
        SERVER_HOST,
        check=True,
    )
    return "runtime-alpha@example.com", "runtime-bravo@example.com"


def install_client_with_two_endpoints(runner) -> tuple[str, str, str]:
    runner(
        "client",
        "install",
        "--path",
        INSTALL_PATH,
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--mode",
        "proxy",
        "--host",
        CLIENT_PRIMARY,
        "--user",
        "runtime-primary@example.com",
        "--password",
        "runtime-primary-pass",
        "--force",
        check=True,
    )
    runner(
        "client",
        "install",
        "--path",
        INSTALL_PATH,
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--mode",
        "proxy",
        "--host",
        CLIENT_SECONDARY,
        "--user",
        "runtime-secondary@example.com",
        "--password",
        "runtime-secondary-pass",
        "--force",
        check=True,
    )
    primary_tag = helpers.expected_proxy_tag(CLIENT_PRIMARY)
    secondary_tag = helpers.expected_proxy_tag(CLIENT_SECONDARY)
    reverse_tag = helpers.expected_reverse_tag("runtime-primary@example.com", CLIENT_PRIMARY)
    add_client_redirects(runner, primary_tag)
    return primary_tag, secondary_tag, reverse_tag


def add_client_redirects(runner, primary_tag: str) -> None:
    runner(
        "client",
        "redirect",
        "add",
        "--path",
        INSTALL_PATH,
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--cidr",
        CLIENT_REDIRECT_CIDR,
        "--tag",
        primary_tag,
        "--no-routes",
        "--quiet",
        check=True,
    )
    runner(
        "client",
        "redirect",
        "add",
        "--path",
        INSTALL_PATH,
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--domain",
        CLIENT_REDIRECT_DOMAIN,
        "--tag",
        primary_tag,
        "--quiet",
        check=True,
    )


def client_desired(host) -> dict:
    return helpers.read_toml(host, helpers.CLIENT_CONFIG_FILE).get("client") or {}


def server_desired(host) -> dict:
    return helpers.read_toml(host, helpers.SERVER_CONFIG_FILE).get("server") or {}


def endpoint_by_tag(state: dict, tag: str) -> dict:
    for endpoint in state.get("endpoints") or []:
        if endpoint.get("tag") == tag:
            return endpoint
    raise AssertionError(f"Endpoint {tag} not found in Desired state")


def redirects_for_tag(state: dict, tag: str) -> list[dict]:
    return [entry for entry in state.get("redirects") or [] if entry.get("outbound_tag") == tag]
