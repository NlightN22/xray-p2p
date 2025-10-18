from __future__ import annotations

from typing import Dict, Tuple


SETUP_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xsetup.sh"
_PROVISIONED: Dict[Tuple[str, str], object] = {}


def run_checked(host, command: str, description: str):
    """Run a shell command via testinfra host and assert a zero return code."""
    result = host.run(command)
    assert result.rc == 0, (
        f"{description} failed with rc={result.rc}\n"
        f"stdout:\n{result.stdout}\n"
        f"stderr:\n{result.stderr}"
    )
    return result


def check_iperf_open(host, label: str, target: str):
    """Verify that iperf3 can reach the target address."""
    command = (
        f"iperf3 -c {target} -t 1 -P 1 >/dev/null 2>&1 && echo open || "
        "{ echo closed; false; }"
    )
    result = run_checked(host, command, f"{label} iperf3 check")
    assert "open" in result.stdout.strip(), (
        f"Expected iperf3 connection to {target} to be open for {label}.\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )


def ensure_stage_one(router_host, user: str, client_lan: str):
    """
    Execute the stage-one provisioning script exactly once per (user, client LAN).
    Returns the cached CommandResult when the script has already been run.
    """
    key = (user, client_lan)
    if key in _PROVISIONED:
        return _PROVISIONED[key]

    status = router_host.run("x2 status")
    if status.rc == 0 and "tunnel" in status.stdout.lower():
        _PROVISIONED[key] = status
        return status

    command = (
        f"curl -fsSL {SETUP_URL} | sh -s -- 10.0.0.1 {user} 10.0.101.0/24 {client_lan}"
    )
    result = run_checked(router_host, command, f"xsetup for {user}")
    combined_output = f"{result.stdout}\n{result.stderr}"
    assert "All steps completed successfully." in combined_output, (
        "xsetup did not report success.\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )

    _PROVISIONED[key] = result
    return result


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
