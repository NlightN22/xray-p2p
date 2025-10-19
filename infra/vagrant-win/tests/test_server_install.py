import pytest

from .helpers import (
    SERVER_CONFIG_DIR,
    load_inbounds_config,
    server_install,
    server_is_installed,
    server_remove,
    start_port_guard,
    stop_port_guard,
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
def test_server_install_basic(host_r1):
    server_remove(host_r1, purge_core=True, check=False)
    assert not server_is_installed(host_r1), "Server should be absent before installation"

    install_env = {
        "XRAY_REISSUE_CERT": "1",
    }

    success_port = "9443"
    server_install(
        host_r1,
        "pytest-auto",
        success_port,
        "--force",
        env={**install_env, "XRAY_SKIP_PORT_CHECK": "1"},
    )
    assert server_is_installed(host_r1), "Server should report as installed"

    service_path = "/etc/init.d/xray-p2p"
    service_file = host_r1.file(service_path)
    assert service_file.exists, "Init script must exist after install"
    stat_result = host_r1.run(f"ls -l {service_path}")
    assert stat_result.rc == 0, "Unable to inspect init script permissions"
    assert "x" in stat_result.stdout.split()[0], "Init script must be executable"

    config_dir = host_r1.file(SERVER_CONFIG_DIR)
    assert config_dir.exists, "Config directory must exist after install"
    inbound_file = host_r1.file(f"{SERVER_CONFIG_DIR}/inbounds.json")
    assert inbound_file.exists, "Inbound config must be seeded after install"

    inbounds = load_inbounds_config(host_r1)
    trojan_port = _extract_trojan_port(inbounds)
    assert trojan_port == int(success_port), f"Trojan inbound should use assigned port {success_port}, got {trojan_port}"

    server_install(
        host_r1,
        "pytest-auto",
        success_port,
        "--force",
        env={**install_env, "XRAY_SKIP_PORT_CHECK": "1"},
        check=False,
    )
    inbounds_after = load_inbounds_config(host_r1)
    assert _extract_trojan_port(inbounds_after) == int(success_port), "Port should remain stable after re-install"

    server_remove(host_r1, purge_core=True)
    assert not server_is_installed(host_r1), "Server should be removed after cleanup"
    assert not host_r1.file(SERVER_CONFIG_DIR).exists, "Config directory should be removed during cleanup"


@pytest.mark.serial
def test_server_install_port_collisions(host_r1):
    server_remove(host_r1, purge_core=True, check=False)
    assert not server_is_installed(host_r1)

    install_env = {
        "XRAY_REISSUE_CERT": "1",
    }

    server_install(host_r1, "pytest-default", "8443", "--force", env=install_env)
    assert server_is_installed(host_r1), "Server should be installed for port discovery"

    inbounds = load_inbounds_config(host_r1)
    trojan_port = _extract_trojan_port(inbounds)
    assert trojan_port is not None, "Trojan inbound port must be present"

    inbound_ports = _collect_inbound_ports(inbounds)
    assert inbound_ports, "Expected non-empty inbound port list"

    server_remove(host_r1, purge_core=True, check=False)
    assert not server_is_installed(host_r1)

    for port in inbound_ports:
        if port == trojan_port:
            continue
        guard_pid = ""
        try:
            guard_pid = start_port_guard(host_r1, port)
            conflict = server_install(
                host_r1,
                "pytest-auto",
                str(trojan_port),
                "--force",
                env=install_env,
                check=False,
                description=f"install server with occupied port {port}",
            )
            combined_output = f"{conflict.stdout}\n{conflict.stderr}"
            assert conflict.rc != 0, f"Install should fail when inbound port {port} is occupied"
            assert str(port) in combined_output, f"Expected collision message to mention port {port}"
        finally:
            stop_port_guard(host_r1, guard_pid)
            server_remove(host_r1, purge_core=True, check=False)
            assert not server_is_installed(host_r1), f"Server should be absent after collision test on port {port}"
