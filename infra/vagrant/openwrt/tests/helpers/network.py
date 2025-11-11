from __future__ import annotations

from .utils import run_checked


def get_interface_ipv4(host, interface: str) -> str:
    """
    Return the first IPv4 address assigned to a given interface on the host.
    """
    result = run_checked(
        host, f"ip -o -4 addr show dev {interface}", f"discover IPv4 on {interface}"
    )
    lines = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    assert lines, f"No IPv4 address reported for interface {interface}."
    first = lines[0].split()
    assert len(first) >= 4, (
        f"Unexpected address output for {interface}.\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )
    ipv4_cidr = first[3]
    address = ipv4_cidr.split("/", 1)[0]
    assert address, (
        f"Failed to parse IPv4 address from interface {interface} output.\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )
    return address


__all__ = ["get_interface_ipv4"]
