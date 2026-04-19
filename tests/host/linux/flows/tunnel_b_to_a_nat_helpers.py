from __future__ import annotations

from dataclasses import dataclass
import re

import pytest

from tests.host.linux import _helpers as helpers


@dataclass(frozen=True, slots=True)
class NatDebugContext:
    client_host: object
    server_host: object
    client_nat_runner: object
    server_nat_runner: object
    nat_snippet: str
    nat_entries: str


def _safe(text: str | None) -> str:
    if not text:
        return ""
    return text.encode("ascii", "ignore").decode()


def detect_chain_cmd(host, *, chain_name: str) -> str | None:
    candidate_chains = (chain_name, "prerouting")
    for table in ("xray_transparent", "fw4"):
        table_list = host.run(f"sudo -n nft list table inet {table}")
        if table_list.rc != 0:
            continue
        for candidate in candidate_chains:
            if re.search(rf"chain\s+{candidate}\b", table_list.stdout or ""):
                return f"sudo -n nft list chain inet {table} {candidate}"
    return None


def nft_counter_sum(host, *, chain_name: str) -> int:
    table_result = host.run("sudo -n nft list table inet xray_transparent")
    if table_result.rc == 0:
        matches = re.findall(r"counter packets\s+(\d+)", table_result.stdout or "")
        return sum(int(value) for value in matches)
    cmd = detect_chain_cmd(host, chain_name=chain_name)
    if not cmd:
        return 0
    result = host.run(cmd)
    if result.rc != 0:
        pytest.fail(
            f"Failed to list nft chain with command: {cmd}\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    matches = re.findall(r"counter packets\s+(\d+)", result.stdout or "")
    return sum(int(value) for value in matches)


def iptables_counter_sum(host) -> int:
    for chain in ("XRAY_TRANSPARENT", "xray_transparent"):
        result = host.run(f"sudo -n /usr/sbin/iptables -t nat -L {chain} -v -n")
        if result.rc != 0:
            continue
        total = 0
        for line in (result.stdout or "").splitlines():
            line = line.strip()
            parts = line.split()
            if len(parts) >= 2 and parts[0].isdigit():
                try:
                    total += int(parts[0])
                except ValueError:
                    continue
        if total:
            return total
    return 0


def traffic_counter_sum(host, *, chain_name: str) -> int:
    nft_total = nft_counter_sum(host, chain_name=chain_name)
    ipt_total = iptables_counter_sum(host)
    return max(nft_total, ipt_total)


def has_nat_chain(host, *, chain_name: str) -> bool:
    cmd = detect_chain_cmd(host, chain_name=chain_name)
    if cmd:
        res = host.run(cmd)
        if res.rc == 0 and ("counter" in (res.stdout or "") or "chain" in (res.stdout or "")):
            return True
    for chain in ("XRAY_TRANSPARENT", "xray_transparent"):
        check = host.run(f"sudo -n /usr/sbin/iptables -t nat -L {chain} -v -n")
        if check.rc == 0 and "Chain" in (check.stdout or ""):
            return True
    return False


def nat_debug(ctx: NatDebugContext) -> str:
    client_nat_dump = ctx.client_nat_runner("nat-redirect", "list", check=False).stdout or ""
    server_nat_dump = ctx.server_nat_runner("nat-redirect", "list", check=False).stdout or ""
    client_chain = ctx.client_host.run("sudo -n nft list table inet xray_transparent")
    server_chain = ctx.server_host.run("sudo -n nft list table inet xray_transparent")
    client_iptables = ctx.client_host.run("sudo -n /usr/sbin/iptables -t nat -L XRAY_TRANSPARENT -v -n")
    server_iptables = ctx.server_host.run("sudo -n /usr/sbin/iptables -t nat -L XRAY_TRANSPARENT -v -n")
    sockets = ctx.client_host.run("sudo -n netstat -lpn 2>/dev/null | grep '51180|51080|52080|62022|62023' || true")
    processes = ctx.client_host.run("ps w | grep -E 'xp2p|xray' | grep -v grep")
    client_xray = ctx.client_host.run(f"cat {helpers.CLIENT_LIVE_DIR / 'xray.json'} 2>/dev/null || true")
    server_xray = ctx.server_host.run(f"cat {helpers.SERVER_LIVE_DIR / 'xray.json'} 2>/dev/null || true")
    client_runtime = ctx.client_host.run(f"cat {helpers.CLIENT_LIVE_DIR / 'runtime.json'} 2>/dev/null || true")
    server_runtime = ctx.server_host.run(f"cat {helpers.SERVER_LIVE_DIR / 'runtime.json'} 2>/dev/null || true")
    client_run_log = ctx.client_host.run("cat /tmp/xp2p-*-run.log 2>/dev/null || true")
    server_run_log = ctx.server_host.run("cat /tmp/xp2p-*-run.log 2>/dev/null || true")
    client_err_log = ctx.client_host.run("cat /var/log/xp2p/*.err 2>/dev/null || true")
    server_err_log = ctx.server_host.run("cat /var/log/xp2p/*.err 2>/dev/null || true")
    client_snippet = ctx.client_host.run(f"sudo -n cat {ctx.nat_snippet} 2>/dev/null || true")
    server_snippet = ctx.server_host.run(f"sudo -n cat {ctx.nat_snippet} 2>/dev/null || true")
    client_entries_ls = ctx.client_host.run(f"sudo -n ls -l {ctx.nat_entries} 2>/dev/null || true")
    server_entries_ls = ctx.server_host.run(f"sudo -n ls -l {ctx.nat_entries} 2>/dev/null || true")
    return (
        f"server_nat:\n{_safe(server_nat_dump)}\nclient_nat:\n{_safe(client_nat_dump)}\n"
        f"server_chain:\n{_safe(server_chain.stdout)}\n{_safe(server_chain.stderr)}\n"
        f"client_chain:\n{_safe(client_chain.stdout)}\n{_safe(client_chain.stderr)}\n"
        f"server_iptables:\n{_safe(server_iptables.stdout)}\n{_safe(server_iptables.stderr)}\n"
        f"client_iptables:\n{_safe(client_iptables.stdout)}\n{_safe(client_iptables.stderr)}\n"
        f"sockets:\n{_safe(sockets.stdout)}\n{_safe(sockets.stderr)}\n"
        f"processes:\n{_safe(processes.stdout)}\n{_safe(processes.stderr)}\n"
        f"client_xray:\n{_safe(client_xray.stdout)}\n{_safe(client_xray.stderr)}\n"
        f"server_xray:\n{_safe(server_xray.stdout)}\n{_safe(server_xray.stderr)}\n"
        f"client_runtime:\n{_safe(client_runtime.stdout)}\n{_safe(client_runtime.stderr)}\n"
        f"server_runtime:\n{_safe(server_runtime.stdout)}\n{_safe(server_runtime.stderr)}\n"
        f"client_run_log:\n{_safe(client_run_log.stdout)}\n{_safe(client_run_log.stderr)}\n"
        f"server_run_log:\n{_safe(server_run_log.stdout)}\n{_safe(server_run_log.stderr)}\n"
        f"client_err_log:\n{_safe(client_err_log.stdout)}\n{_safe(client_err_log.stderr)}\n"
        f"server_err_log:\n{_safe(server_err_log.stdout)}\n{_safe(server_err_log.stderr)}\n"
        f"client_snippet:\n{_safe(client_snippet.stdout)}\n{_safe(client_snippet.stderr)}\n"
        f"server_snippet:\n{_safe(server_snippet.stdout)}\n{_safe(server_snippet.stderr)}\n"
        f"client_entries_ls:\n{_safe(client_entries_ls.stdout)}\n{_safe(client_entries_ls.stderr)}\n"
        f"server_entries_ls:\n{_safe(server_entries_ls.stdout)}\n{_safe(server_entries_ls.stderr)}\n"
    )
