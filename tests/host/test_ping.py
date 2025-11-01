import pytest


@pytest.mark.host
def test_xp2p_service_ping(xp2p_server_service, xp2p_client_runner, xp2p_options):
    """Клиентский xp2p ping должен видеть поднятый на сервере сервис."""
    result = xp2p_client_runner(
        "ping",
        xp2p_options["target"],
        "--port",
        str(xp2p_options["port"]),
        "--count",
        str(xp2p_options["attempts"]),
    )

    assert result.rc == 0, (
        "xp2p ping завершился с ошибкой:\n"
        f"STDOUT:\n{result.stdout}\n"
        f"STDERR:\n{result.stderr}"
    )

    output_lower = (result.stdout or "").lower()
    if "100% loss" in output_lower:
        pytest.fail(
            "xp2p ping сообщил о 100% потерь:\n"
            f"{result.stdout}"
        )

    if (result.stderr or "").strip():
        pytest.fail(
            "xp2p ping вывел ошибки в STDERR:\n"
            f"{result.stderr}"
        )
