import shlex

import pytest

from .helpers import (
    SERVER_CONFIG_DIR,
    load_inbounds_config,
    server_cert_apply,
    server_cert_selfsigned,
    server_install,
    server_is_installed,
    server_remove,
)


def _extract_certificates(inbounds: dict) -> tuple[str | None, str | None]:
    for inbound in inbounds.get("inbounds", []):
        if not isinstance(inbound, dict):
            continue
        if inbound.get("protocol") != "trojan":
            continue
        stream = inbound.get("streamSettings", {})
        if not isinstance(stream, dict):
            continue
        tls_settings = stream.get("tlsSettings", {})
        if not isinstance(tls_settings, dict):
            continue
        certificates = tls_settings.get("certificates", [])
        if not certificates:
            continue
        first = certificates[0]
        if isinstance(first, dict):
            return first.get("certificateFile"), first.get("keyFile")
    return None, None


@pytest.mark.serial
def test_server_certificate_helpers(host_r1):
    server_remove(host_r1, purge_core=True, check=False)
    assert not server_is_installed(host_r1), "Server must be absent before provisioning"

    install_env = {
        "XRAY_REISSUE_CERT": "1",
        "XRAY_SKIP_PORT_CHECK": "1",
    }

    custom_port = "9555"
    try:
        server_install(host_r1, "pytest-cert", custom_port, "--force", env=install_env)
        assert server_is_installed(host_r1), "Server should report as installed"
        config_dir = host_r1.file(SERVER_CONFIG_DIR)
        assert config_dir.exists, "Config directory must exist after installation"

        inbounds_initial = load_inbounds_config(host_r1)
        cert_path, key_path = _extract_certificates(inbounds_initial)
        assert cert_path, "Trojan inbound must define certificate path"
        assert key_path, "Trojan inbound must define key path"

        hostname = "pytest-cert.local"
        server_cert_selfsigned(
            host_r1,
            env={
                "XRAY_REISSUE_CERT": "1",
                "XRAY_SERVER_NAME": hostname,
            },
        )

        cert_file = host_r1.file(cert_path)
        key_file = host_r1.file(key_path)
        assert cert_file.exists, f"Self-signed certificate missing at {cert_path}"
        assert key_file.exists, f"Self-signed key missing at {key_path}"
        cert_nonempty = host_r1.run(f"test -s {shlex.quote(cert_path)}")
        key_nonempty = host_r1.run(f"test -s {shlex.quote(key_path)}")
        assert cert_nonempty.rc == 0, "Certificate file should be non-empty"
        assert key_nonempty.rc == 0, "Key file should be non-empty"

        subject_result = host_r1.run(
            f"openssl x509 -in {shlex.quote(cert_path)} -noout -subject"
        )
        assert subject_result.rc == 0, (
            f"Unable to read certificate subject\nstdout:\n{subject_result.stdout}\n"
            f"stderr:\n{subject_result.stderr}"
        )
        assert hostname in subject_result.stdout, "Certificate subject should contain requested hostname"

        alt_cert = "/tmp/pytest-alt-cert.pem"
        alt_key = "/tmp/pytest-alt-key.pem"
        subject = "/CN=pytest-alt.local"
        jq_check = host_r1.run("command -v jq")
        assert jq_check.rc == 0, (
            "jq binary is required on server host for certificate tests\n"
            f"stdout:\n{jq_check.stdout}\n"
            f"stderr:\n{jq_check.stderr}"
        )
        generate_result = host_r1.run(
            " ".join(
                [
                    "openssl",
                    "req",
                    "-x509",
                    "-nodes",
                    "-newkey",
                    "rsa:2048",
                    "-days",
                    "180",
                    "-keyout",
                    shlex.quote(alt_key),
                    "-out",
                    shlex.quote(alt_cert),
                    "-subj",
                    shlex.quote(subject),
                ]
            )
        )
        assert generate_result.rc == 0, (
            f"Failed to mint alternate certificate\nstdout:\n{generate_result.stdout}\n"
            f"stderr:\n{generate_result.stderr}"
        )

        apply_result = server_cert_apply(
            host_r1,
            alt_cert,
            alt_key,
            check=False,
            description="apply alternate TLS material",
        )
        if apply_result.rc != 0:
            pytest.fail(
                "Certificate apply helper failed\n"
                f"stdout:\n{apply_result.stdout}\n"
                f"stderr:\n{apply_result.stderr}"
            )
        combined_apply = f"{apply_result.stdout}\n{apply_result.stderr}"
        assert "Applied certificate" in combined_apply, "Expected confirmation message after applying TLS material"

        inbounds_after = load_inbounds_config(host_r1)
        cert_after, key_after = _extract_certificates(inbounds_after)
        assert cert_after == alt_cert, "Inbound certificate path should be updated to alternate material"
        assert key_after == alt_key, "Inbound key path should be updated to alternate material"

        validation_result = host_r1.run(
            f"openssl x509 -in {shlex.quote(cert_after)} -noout -subject"
        )
        assert validation_result.rc == 0, (
            f"Alternate certificate should be readable\nstdout:\n{validation_result.stdout}\n"
            f"stderr:\n{validation_result.stderr}"
        )
        assert "pytest-alt.local" in validation_result.stdout, "Alternate certificate subject mismatch"
    finally:
        for leftover in ("/tmp/pytest-alt-cert.pem", "/tmp/pytest-alt-key.pem"):
            host_r1.run(f"rm -f {shlex.quote(leftover)}")
        server_remove(host_r1, purge_core=True, check=False)

    assert not server_is_installed(host_r1), "Server should be removed after cleanup"
    assert not host_r1.file(SERVER_CONFIG_DIR).exists, "Server config directory should not remain after cleanup"
