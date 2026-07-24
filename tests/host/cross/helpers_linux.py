from __future__ import annotations

import time
from pathlib import PurePosixPath

import pytest
from tests.host import cli_json
from testinfra.host import Host

from tests.host.linux import _helpers as linux_helpers
from tests.host.linux import env as linux_env
from tests.host.cross.helpers_common import extract_marker


def linux_runner(host: Host):
    def _runner(*args: str, check: bool = False):
        cmd = list(args)
        if len(cmd) >= 2 and cmd[0] in {"client", "server"} and cmd[1] == "remove":
            if "--quiet" not in cmd:
                cmd.append("--quiet")
        result = linux_env.run_xp2p(host, *cmd)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner


def start_linux_client_deploy(
    host: Host,
    *,
    log_path: PurePosixPath,
    remote_host: str,
    deploy_port: str | None = None,
    trojan_user: str,
    trojan_password: str,
    trojan_port: str,
    install_dir: str | None = None,
) -> int:
    args = [
        log_path.as_posix(),
        remote_host,
        trojan_user,
        trojan_password,
        trojan_port,
    ]
    if deploy_port:
        args.insert(2, deploy_port)
    if install_dir:
        args += ["--install-dir", install_dir]
    result = linux_env.run_guest_script(host, "scripts/linux/start_xp2p_client_deploy.sh", *args)
    if result.rc != 0:
        pytest.fail(
            "Failed to start xp2p client deploy.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid = extract_marker(result.stdout, "__XP2P_PID__=")
    if not pid:
        pytest.fail(
            "xp2p client deploy script did not emit PID marker.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return int(pid)


def start_linux_server_deploy(
    host: Host,
    *,
    log_path: PurePosixPath,
    listen_addr: str | None = None,
    deploy_link: str,
) -> int:
    args = [log_path.as_posix()]
    if listen_addr:
        args.append(listen_addr)
    args.append(deploy_link)
    result = linux_env.run_guest_script(host, "scripts/linux/start_xp2p_server_deploy.sh", *args)
    if result.rc != 0:
        pytest.fail(
            "Failed to start xp2p server deploy.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid = extract_marker(result.stdout, "__XP2P_PID__=")
    if not pid:
        pytest.fail(
            "xp2p server deploy script did not emit PID marker.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return int(pid)


def read_optional_linux_log(host: Host, path: PurePosixPath) -> str:
    result = linux_env.run_guest_script(host, "scripts/linux/read_file.sh", path.as_posix())
    if result.rc == 0:
        return result.stdout or ""
    if result.rc == 3:
        return ""
    pytest.fail(
        f"Failed to read log {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def wait_for_log_value_linux(
    host: Host,
    path: PurePosixPath,
    *,
    extractor,
    description: str,
    timeout: int,
):
    deadline = time.time() + timeout
    last_text = ""
    while time.time() < deadline:
        text = read_optional_linux_log(host, path)
        if text:
            value = extractor(text)
            if value:
                return value
            last_text = text
        time.sleep(1)
    tail = "\n".join((last_text or "").splitlines()[-30:])
    pytest.fail(f"Timed out waiting for {description}. Recent log tail:\n{tail}")


def wait_for_log_phrase_linux(host: Host, path: PurePosixPath, phrase: str, *, timeout: int) -> None:
    expected_variants = (phrase, f"{phrase}")

    def _matcher(text: str) -> bool | None:
        for variant in expected_variants:
            if variant in text:
                return True
        return None

    wait_for_log_value_linux(
        host,
        path,
        extractor=_matcher,
        description=f"'{phrase}' in {path}",
        timeout=timeout,
    )


def wait_for_client_link_linux(host: Host, log_path: PurePosixPath, *, timeout: int) -> str:
    def _parse_json_link(text: str) -> str | None:
        return cli_json.link(text)
    link = wait_for_log_value_linux(
        host,
        log_path,
        extractor=_parse_json_link,
        description="xp2p client deploy link",
        timeout=timeout,
    )
    if not link:
        pytest.fail("xp2p client deploy log did not include a deploy link")
    return link


def wait_for_error_phrase_linux(host: Host, path: PurePosixPath, phrase: str, *, timeout: int) -> None:
    def _matcher(text: str) -> bool | None:
        if phrase in text:
            return True
        return None

    wait_for_log_value_linux(
        host,
        path,
        extractor=_matcher,
        description=f"'{phrase}' in {path}",
        timeout=timeout,
    )


def detect_linux_ipv4_non_nat(host: Host) -> str:
    command = "ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1"
    result = host.run(command)
    if result.rc != 0:
        pytest.fail(
            "Failed to detect IPv4 addresses.\n"
            f"CMD: {command}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not addresses:
        pytest.fail("No IPv4 addresses found on host")
    for addr in addresses:
        if not addr.startswith("10.0.2."):
            return addr
    return addresses[0]


def reset_linux_logs(host: Host, path: PurePosixPath) -> None:
    linux_helpers.remove_path(host, path)


def assert_linux_server_install_dir(host: Host, install_dir: PurePosixPath) -> None:
    state_path = linux_helpers.SERVER_APPLIED_STATE_FILE
    config_dir = install_dir / linux_helpers.SERVER_CONFIG_DIR_NAME
    assert linux_helpers.path_exists(host, state_path), f"server install state missing: {state_path}"
    assert linux_helpers.path_exists(host, config_dir), f"server config dir missing: {config_dir}"
