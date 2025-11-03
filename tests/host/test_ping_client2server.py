import pytest

from tests.host import _env

CLIENT_SUBNET_HOST = "10.0.10.20"
FIREWALL_RULE_NAME = "xp2p-test-block-client"
FIREWALL_PROFILES = "Domain,Private,Public"


def _assert_ping_success(result) -> None:
    assert result.rc == 0, (
        "xp2p ping failed:\n"
        f"STDOUT:\n{result.stdout}\n"
        f"STDERR:\n{result.stderr}"
    )

    output_lower = (result.stdout or "").lower()
    if "100% loss" in output_lower:
        pytest.fail(
            "xp2p ping reported 100% packet loss:\n"
            f"{result.stdout}"
        )

    stderr_text = (result.stderr or "").strip()
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
    result = _env.run_guest_script(
        server_host,
        "scripts/configure_firewall_rule.ps1",
        Name=FIREWALL_RULE_NAME,
        RemoteAddress=remote_address,
        LocalPort=str(port),
        Ensure=ensure,
        Protocol="TCP",
    )
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
def test_xp2p_service_ping(xp2p_server_service, xp2p_client_runner, xp2p_options):
    """Verify that the client xp2p ping reaches the server-side diagnostics service."""
    result = _run_ping(xp2p_client_runner, xp2p_options)
    _assert_ping_success(result)


@pytest.mark.host
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
