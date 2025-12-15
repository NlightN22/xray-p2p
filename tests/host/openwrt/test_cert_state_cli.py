from __future__ import annotations

import textwrap

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env


def _xp2p(host, *args: str, check: bool = False):
    result = openwrt_env.run_xp2p(host, *args)
    if check and result.rc != 0:
        pytest.fail(
            f"xp2p command failed (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_cert_state_reports_valid_and_expired(openwrt_server_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    run = lambda *cmd, check=False: _xp2p(openwrt_server_host, *cmd, check=check)
    install_dir = helpers.INSTALL_ROOT.as_posix()
    config_dir = helpers.SERVER_CONFIG_DIR_NAME
    cert_path = helpers.SERVER_CONFIG_DIR / "cert.pem"
    key_path = helpers.SERVER_CONFIG_DIR / "key.pem"
    server_host = "certstate.openwrt.test"

    try:
        helpers.cleanup_server_install(openwrt_server_host, run)

        run(
            "server",
            "install",
            "--path",
            install_dir,
            "--config-dir",
            config_dir,
            "--host",
            server_host,
            "--force",
            check=True,
        )

        state = run(
            "server",
            "cert",
            "state",
            "--path",
            install_dir,
            "--config-dir",
            config_dir,
        )
        assert state.rc == 0, f"expected successful cert state, rc={state.rc}\nSTDOUT:\n{state.stdout}\nSTDERR:\n{state.stderr}"
        assert "Status:      OK" in state.stdout, f"missing OK status\n{state.stdout}"
        assert f"Subject:     CN={server_host}" in state.stdout, f"missing subject\n{state.stdout}"
        assert f"Certificate: {cert_path.as_posix()}" in state.stdout, f"missing cert path\n{state.stdout}"
        assert f"Key:         {key_path.as_posix()}" in state.stdout, f"missing key path\n{state.stdout}"

        helpers.write_text(openwrt_server_host, cert_path, _EXPIRED_CERT)
        helpers.write_text(openwrt_server_host, key_path, _EXPIRED_KEY)

        expired = run(
            "server",
            "cert",
            "state",
            "--path",
            install_dir,
            "--config-dir",
            config_dir,
        )
        assert expired.rc == 1, f"expected failure rc for expired cert, got {expired.rc}\nSTDOUT:\n{expired.stdout}\nSTDERR:\n{expired.stderr}"
        assert "Status:      EXPIRED" in expired.stdout, f"missing expired status\n{expired.stdout}"
    finally:
        run(
            "server",
            "remove",
            "--path",
            install_dir,
            "--config-dir",
            config_dir,
            "--ignore-missing",
            "--quiet",
        )


_EXPIRED_CERT = textwrap.dedent(
    """
    -----BEGIN CERTIFICATE-----
    MIICAjCCAWugAwIBAgIBAjANBgkqhkiG9w0BAQsFADAhMR8wHQYDVQQDExZjZXJ0
    c3RhdGUub3BlbndydC50ZXN0MB4XDTIwMDEwMTAwMDAwMFoXDTIwMDEwMjAwMDAw
    MFowITEfMB0GA1UEAxMWY2VydHN0YXRlLm9wZW53cnQudGVzdDCBnzANBgkqhkiG
    9w0BAQEFAAOBjQAwgYkCgYEAqf6OHRSf2LX93CvtvnbjSEd6R1Y+ySraSESliQ9/
    jM62PiWt222Pj3xmzPla8PJ0xUplMj463WZwIgK2tRZdGGRpFqMVvq01nC2rGhil
    C8m+O71CvYgVkBqManCENkC9PinThG0yCJj62KlD2fqEY9CWFmfPzBHpDeJdGu0b
    d30CAwEAAaNKMEgwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMB
    MCEGA1UdEQQaMBiCFmNlcnRzdGF0ZS5vcGVud3J0LnRlc3QwDQYJKoZIhvcNAQEL
    BQADgYEAOJ458YPlwecexfd50I1RbI9ggXmjzo2cLieKzBvP/p+evLrdUm8Cufp4
    WYMKwBfb52sgUp9iT1j6wVemXNmuxOm7tNLZ2vFBOQAwCD+oKq0qy+OuV0IeZiF1
    CrWXsHLvE37zMVcUGfFYFafG+2s29a5G3ex55wamW+nb/uXM060=
    -----END CERTIFICATE-----
    """
).strip()

_EXPIRED_KEY = textwrap.dedent(
    """
    -----BEGIN RSA PRIVATE KEY-----
    MIICXAIBAAKBgQCp/o4dFJ/Ytf3cK+2+duNIR3pHVj7JKtpIRKWJD3+MzrY+Ja3b
    bY+PfGbM+Vrw8nTFSmUyPjrdZnAiAra1Fl0YZGkWoxW+rTWcLasaGKULyb47vUK9
    iBWQGoxqcIQ2QL0+KdOEbTIImPrYqUPZ+oRj0JYWZ8/MEekN4l0a7Rt3fQIDAQAB
    AoGAKF5NyzQZnXHiXgWEiKVc5c4riIM/l6/4dA7xLHIkvQBdoLZ76c7Dt7Q4CVbx
    tKQu/KblDyBeBDOOT1VLpAcyhfQ4/blcduC/paklH3Z8P5tMcburKev0S+eT/cYk
    +yVE+8iXWKH8fV4Gs75Vbpl8awxKHNyG2WQQCydxS/CjCjUCQQDXb2/gCzrllX99
    zHl8wyvhNYVnUkGKcfIWmvNkhCPoNWibbK6ZHf/Y9v6uAgLfRlRKtbjwwus65k6X
    38ibtFIHAkEAygC97uASW8q1fNj5q/VAGDxvu8uDW60PuzTNc+Gm9fscHTqEal+M
    BfCTpNA56yT7sKAcnIr/BpdhGC4lGZt5WwJBAKMvTATfPMuuxBWcDuIMTG6YxeYP
    jom56fBpire2yCQaYJRqbI6bBLNp1FwmNdq+QRceM2pbmybQUPQFlMUsf30CQA8T
    TRl1uYkGMNM3cjKmI/lrET+nqY7+9GyZPTgHwCkda3S2+EjkBpQu5yXmsFvfL7V3
    zYrVSMEaLRHb58Loen8CQFYJZm7M7b9XOFbDRydhwyxaRBp5nwaCCP8U8+83NoJb
    WWZmSK6tCVZDeebdvTNp300uZHci5l3v0UTgQdjF95U=
    -----END RSA PRIVATE KEY-----
    """
).strip()
