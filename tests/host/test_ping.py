import pytest


@pytest.mark.host
def test_xp2p_service_ping(xp2p_server_service, xp2p_client_runner, xp2p_options):
    """Verify that the client xp2p ping reaches the server-side diagnostics service."""
    result = xp2p_client_runner(
        "ping",
        xp2p_options["target"],
        "--port",
        str(xp2p_options["port"]),
        "--count",
        str(xp2p_options["attempts"]),
    )

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
