import pytest


@pytest.mark.host
def test_xp2p_guest_ping(go_guest_runner, xp2p_options):
    """Verify xp2p guest ping succeeds against the server endpoint."""
    result = go_guest_runner(
        "tests/guest/ping.go",
        target=xp2p_options["target"],
        port=xp2p_options["port"],
        attempts=xp2p_options["attempts"],
    )

    assert result.returncode == 0, (
        "Guest ping command failed:\n"
        f"STDOUT:\n{result.stdout}\n"
        f"STDERR:\n{result.stderr}"
    )

    output_lower = result.stdout.lower()
    if "100% packet loss" in output_lower:
        pytest.fail(
            "Guest ping reported 100% packet loss:\n"
            f"{result.stdout}"
        )

    if "error" in output_lower:
        pytest.fail(
            "Guest ping output contains errors:\n"
            f"{result.stdout}"
        )
