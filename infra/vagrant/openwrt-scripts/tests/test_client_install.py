import pytest

from .helpers import (
    SERVER_CONFIG_DIR,
    client_install,
    client_is_installed,
    client_remove,
    load_inbounds_config,
    start_port_guard,
    stop_port_guard,
)

DUMMY_TROJAN_URL = (
    "trojan://pytest-pass@example.com:10443"
    "?security=tls&allowInsecure=1#pytest-client"
)


def _collect_inbound_ports(inbounds: dict) -> list[int]:
    ports: set[int] = set()
    for inbound in inbounds.get("inbounds", []):
        if not isinstance(inbound, dict):
            continue
        raw_port = inbound.get("port")
        if isinstance(raw_port, int):
            ports.add(raw_port)
        elif isinstance(raw_port, str) and raw_port.isdigit():
            ports.add(int(raw_port))
    return sorted(ports)


@pytest.mark.serial
def test_client_install_basic(host_r2):
    client_remove(host_r2, purge_core=True, check=False)
    assert not client_is_installed(host_r2), "Client should be absent before installation"

    base_env = {
        "XRAY_REISSUE_CERT": "1",
    }
    install_env = {**base_env, "XRAY_SKIP_PORT_CHECK": "1"}

    client_install(host_r2, DUMMY_TROJAN_URL, "--force", env=install_env)
    assert client_is_installed(host_r2), "Client should report as installed"

    service_path = "/etc/init.d/xray-p2p"
    service_file = host_r2.file(service_path)
    assert service_file.exists, "Init script must exist after install"
    stat_result = host_r2.run(f"ls -l {service_path}")
    assert stat_result.rc == 0, "Unable to inspect init script permissions"
    assert "x" in stat_result.stdout.split()[0], "Init script must be executable"

    config_dir = host_r2.file(SERVER_CONFIG_DIR)
    assert config_dir.exists, "Config directory must exist after install"
    inbound_file = host_r2.file(f"{SERVER_CONFIG_DIR}/inbounds.json")
    assert inbound_file.exists, "Inbound config must be seeded after install"

    inbounds = load_inbounds_config(host_r2)
    inbound_ports = _collect_inbound_ports(inbounds)
    assert set(inbound_ports) >= {48044, 51080}, f"Unexpected inbound ports: {inbound_ports}"

    client_install(host_r2, DUMMY_TROJAN_URL, "--force", env=install_env)
    inbounds_after = load_inbounds_config(host_r2)
    assert _collect_inbound_ports(inbounds_after) == inbound_ports, "Inbound ports changed after reinstall"

    client_remove(host_r2, purge_core=True)
    assert not client_is_installed(host_r2), "Client should be removed after cleanup"
    assert not host_r2.file(SERVER_CONFIG_DIR).exists, "Config directory should be removed during cleanup"


@pytest.mark.serial
def test_client_install_port_collisions(host_r2):
    client_remove(host_r2, purge_core=True, check=False)
    assert not client_is_installed(host_r2)

    base_env = {
        "XRAY_REISSUE_CERT": "1",
    }
    install_env = {**base_env, "XRAY_SKIP_PORT_CHECK": "1"}

    client_install(host_r2, DUMMY_TROJAN_URL, "--force", env=install_env)
    assert client_is_installed(host_r2), "Client should be installed for port discovery"

    inbounds = load_inbounds_config(host_r2)
    inbound_ports = _collect_inbound_ports(inbounds)

    client_remove(host_r2, purge_core=True, check=False)
    assert not client_is_installed(host_r2)

    for port in inbound_ports:
        guard_pid = ""
        try:
            guard_pid = start_port_guard(host_r2, port)
            conflict = client_install(
                host_r2,
                DUMMY_TROJAN_URL,
                "--force",
                env=base_env,
                check=False,
                description=f"install client with occupied port {port}",
            )
            combined_output = f"{conflict.stdout}\n{conflict.stderr}"
            assert conflict.rc != 0, f"Install should fail when inbound port {port} is occupied"
            assert str(port) in combined_output, f"Expected collision message to mention port {port}"
        finally:
            stop_port_guard(host_r2, guard_pid)
            client_remove(host_r2, purge_core=True, check=False)
            assert not client_is_installed(host_r2), f"Client should be absent after collision test on port {port}"
