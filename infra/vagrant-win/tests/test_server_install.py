import pytest

from .helpers import (
    SERVER_CONFIG_DIR,
    load_inbounds_config,
    server_install,
    server_is_installed,
    server_remove,
)


def _extract_trojan_port(inbounds: dict) -> int | None:
    for inbound in inbounds.get("inbounds", []):
        if not isinstance(inbound, dict):
            continue
        if inbound.get("protocol") != "trojan":
            continue
        port = inbound.get("port")
        if isinstance(port, int):
            return port
        if isinstance(port, str) and port.isdigit():
            return int(port)
    return None


@pytest.mark.serial
def test_server_install_lifecycle(host_r1):
    # Ensure we always start from a clean state.
    server_remove(host_r1, purge_core=True, check=False)
    assert not server_is_installed(host_r1), "Server should be absent before installation"

    guard_pid = None
    try:
        occupy = host_r1.run(
            "sh -c 'busybox nc -l -p 443 >/dev/null 2>&1 & echo $!'"
        )
        assert occupy.rc == 0, f"Failed to start port guard: {occupy.stderr}"
        guard_pid = occupy.stdout.strip()
        assert guard_pid, "Port guard PID missing"

        install_env = {
            "XRAY_FORCE_CONFIG": "1",
            "XRAY_REISSUE_CERT": "1",
        }
        conflict = server_install(
            host_r1,
            "pytest-auto",
            "443",
            env=install_env,
            check=False,
            description="install server on occupied port",
        )
        combined_output = f"{conflict.stdout}\n{conflict.stderr}"
        assert conflict.rc != 0, "Install should fail when target port is occupied"
        assert "Required port(s) already in use" in combined_output, combined_output
    finally:
        if guard_pid:
            host_r1.run(f"kill {guard_pid} >/dev/null 2>&1 || true")
            host_r1.run("sleep 1")

    assert not server_is_installed(host_r1), "Server should still be absent after failed install"

    install_env = {
        "XRAY_FORCE_CONFIG": "1",
        "XRAY_REISSUE_CERT": "1",
    }

    server_install(host_r1, "pytest-auto", "9443", env=install_env)
    assert server_is_installed(host_r1), "Server should report as installed"

    service_file = host_r1.file("/etc/init.d/xray-p2p")
    assert service_file.exists, "Init script must exist after install"
    assert service_file.mode is not None and service_file.mode & 0o111, "Init script must be executable"

    config_dir = host_r1.file(SERVER_CONFIG_DIR)
    assert config_dir.exists, "Config directory must exist after install"
    inbound_file = host_r1.file(f"{SERVER_CONFIG_DIR}/inbounds.json")
    assert inbound_file.exists, "Inbound config must be seeded after install"

    inbounds = load_inbounds_config(host_r1)
    trojan_port = _extract_trojan_port(inbounds)
    assert trojan_port == 9443, f"Trojan inbound should use assigned port 9443, got {trojan_port}"

    # Re-running install should be safe and keep configuration consistent.
    server_install(host_r1, "pytest-auto", "9443", env=install_env)
    inbounds_after = load_inbounds_config(host_r1)
    assert _extract_trojan_port(inbounds_after) == 9443, "Port should remain stable after re-install"

    # Final cleanup to keep environment ready for follow-up tests.
    server_remove(host_r1, purge_core=True)
    assert not server_is_installed(host_r1), "Server should be removed after cleanup"
    assert not host_r1.file(SERVER_CONFIG_DIR).exists, "Config directory should be removed during cleanup"
