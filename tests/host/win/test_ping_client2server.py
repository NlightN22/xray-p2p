from pathlib import Path

import pytest

from tests.host.win import env as _env

CLIENT_SUBNET_HOST = "10.62.10.22"
FIREWALL_RULE_NAME = "xp2p-test-block-client"
FIREWALL_PROFILES = "Domain,Private,Public"
NET_LOG_DIR = Path(r"C:\xp2p\build\logs\win")


def _dump_net_state(host, label: str) -> str:
    path = NET_LOG_DIR / f"ping-client2server-{label}.log"
    result = _env.run_guest_script(
        host,
        "scripts/dump_net_state.ps1",
        OutputPath=str(path),
        Label=label,
    )
    if result.rc != 0:
        return (
            f"Failed to dump net state ({label}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return _env.read_text(host, path)


def _assert_ping_success(result, *, server_host, client_host) -> None:
    output_lower = (result.stdout or "").lower()
    stderr_text = (result.stderr or "").strip()

    if result.rc != 0 or "100% loss" in output_lower:
        server_state = _dump_net_state(server_host, "server")
        client_state = _dump_net_state(client_host, "client")
        pytest.fail(
            "xp2p ping failed:\n"
            f"STDOUT:\n{result.stdout}\n"
            f"STDERR:\n{result.stderr}\n"
            "Server net state:\n"
            f"{server_state}\n"
            "Client net state:\n"
            f"{client_state}"
        )

    if stderr_text:
        stderr_lower = stderr_text.lower()
        if stderr_lower.startswith("#< clixml"):
            if "level=error" in stderr_lower or "level=warn" in stderr_lower:
                pytest.fail(
                    "xp2p ping reported warnings/errors in STDERR:\n"
                    f"{result.stderr}"
                )
        else:
            pytest.fail(
                "xp2p ping wrote unexpected output to STDERR:\n"
                f"{result.stderr}"
            )


def _set_firewall_rule(server_host, *, ensure: str, remote_address: str, port: int) -> None:
    script = f"""
$ErrorActionPreference = 'Stop'
$name = {_env.ps_quote(FIREWALL_RULE_NAME)}
$remote = {_env.ps_quote(remote_address)}
$port = {port}
$ensure = {_env.ps_quote(ensure)}
Get-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue | ForEach-Object {{
    Remove-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue
}}
if ($ensure -eq 'Present') {{
    New-NetFirewallRule `
        -DisplayName $name `
        -Direction Inbound `
        -Action Block `
        -Protocol TCP `
        -LocalPort $port `
        -RemoteAddress $remote `
        -Profile Any `
        -EdgeTraversalPolicy Block | Out-Null
}}
exit 0
"""
    result = _env.run_powershell(server_host, script, label="set_firewall_rule")
    if result.rc != 0:
        pytest.fail(
            f"Failed to set firewall rule Ensure={ensure} on server:\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _run_ping(xp2p_client_runner, xp2p_options):
    return xp2p_client_runner(
        "ping",
        xp2p_options["target"],
        "--port",
        str(xp2p_options["port"]),
        "--count",
        str(xp2p_options["attempts"]),
        )


def _set_firewall_profiles(server_host, *, enabled: bool) -> None:
    state = "Enable" if enabled else "Disable"
    result = _env.run_guest_script(
        server_host,
        "scripts/set_firewall_profiles.ps1",
        State=state,
        Profiles=FIREWALL_PROFILES,
    )
    if result.rc != 0:
        pytest.fail(
            f"Failed to set firewall profiles State={state} on server:\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.win
def test_xp2p_service_ping(
    xp2p_server_service,
    xp2p_client_runner,
    xp2p_options,
    server_host,
    client_host,
):
    """Verify that the client xp2p ping reaches the server-side diagnostics service."""
    result = _run_ping(xp2p_client_runner, xp2p_options)
    _assert_ping_success(result, server_host=server_host, client_host=client_host)


@pytest.mark.host
@pytest.mark.win
def test_xp2p_service_ping_blocked_by_firewall(
    xp2p_server_service, xp2p_client_runner, xp2p_options, server_host
):
    """Ensure the diagnostics ping fails when server firewall blocks the client."""
    port = xp2p_options["port"]
    _set_firewall_profiles(server_host, enabled=True)
    try:
        _set_firewall_rule(
            server_host,
            ensure="Present",
            remote_address=CLIENT_SUBNET_HOST,
            port=port,
        )
        result = _run_ping(xp2p_client_runner, xp2p_options)
        output_lower = (result.stdout or "").lower()
        if result.rc == 0 and "100% loss" not in output_lower:
            pytest.fail(
                "xp2p ping unexpectedly succeeded despite firewall block:\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
    finally:
        _set_firewall_rule(
            server_host,
            ensure="Absent",
            remote_address=CLIENT_SUBNET_HOST,
            port=port,
        )
        _set_firewall_profiles(server_host, enabled=False)
