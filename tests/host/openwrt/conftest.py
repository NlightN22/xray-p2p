from __future__ import annotations

import os
import time
import shlex
import subprocess
from pathlib import PurePosixPath

import pytest
from testinfra.host import Host

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env


@pytest.fixture(scope="session")
def ipk_builder_host() -> Host:
    openwrt_env.require_ipk_builder_environment()
    return openwrt_env.get_ipk_builder_host()


@pytest.fixture(scope="session")
def openwrt_host_factory():
    openwrt_env.require_openwrt_environment()
    return openwrt_env.host_factory()


@pytest.fixture(scope="session")
def alpine_host_factory():
    openwrt_env.require_openwrt_environment()
    return openwrt_env.alpine_host_factory()


@pytest.fixture(scope="session")
def openwrt_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.DEFAULT_OPENWRT_MACHINE)


@pytest.fixture(scope="session")
def openwrt_server_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.OPENWRT_MACHINES[0])


@pytest.fixture(scope="session")
def openwrt_client_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.OPENWRT_MACHINES[1])


@pytest.fixture(scope="session")
def alpine_c1_host(alpine_host_factory) -> Host:
    return alpine_host_factory(openwrt_env.ALPINE_MACHINES[0])


@pytest.fixture(scope="session")
def alpine_c2_host(alpine_host_factory) -> Host:
    return alpine_host_factory(openwrt_env.ALPINE_MACHINES[1])


@pytest.fixture(scope="session")
def openwrt_ipk_target() -> str:
    return openwrt_env.resolve_target_from_env()


@pytest.fixture(scope="session")
def xp2p_openwrt_ipk(openwrt_ipk_target):
    _ensure_host_linux_amd64_xp2p()
    openwrt_env.IPK_OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    skip_build = os.environ.get("XP2P_OPENWRT_SKIP_IPK_BUILD", "").strip().lower() in {"1", "true", "yes"}
    if skip_build:
        artifact = openwrt_env.latest_local_ipk()
        assert artifact, "Expected build/ipk to contain an xp2p ipk when XP2P_OPENWRT_SKIP_IPK_BUILD is set"
        print("TIMING: openwrt ipk build skipped (env XP2P_OPENWRT_SKIP_IPK_BUILD)")
    else:
        openwrt_env.require_ipk_builder_environment()
        ipk_builder_host = openwrt_env.get_ipk_builder_host()
        build_start = time.perf_counter()
        openwrt_env.build_ipk(ipk_builder_host, openwrt_ipk_target)
        build_elapsed = time.perf_counter() - build_start
        print(f"TIMING: openwrt ipk build: {build_elapsed:.2f}s")
        artifact = openwrt_env.latest_local_ipk()
        assert artifact, "Expected build/ipk to contain a freshly built xp2p ipk"
    openwrt_env.ensure_packages_index_present()
    skip_sync = os.environ.get("XP2P_OPENWRT_SKIP_IPK_SYNC", "").strip().lower() in {"1", "true", "yes"}
    if not skip_sync:
        sync_start = time.perf_counter()
        for machine in openwrt_env.OPENWRT_MACHINES:
            openwrt_env.sync_build_output(machine)
        sync_elapsed = time.perf_counter() - sync_start
        print(f"TIMING: openwrt build output sync: {sync_elapsed:.2f}s")
    else:
        print("TIMING: openwrt build output sync skipped (env XP2P_OPENWRT_SKIP_IPK_SYNC)")
    return artifact


def _ensure_host_linux_amd64_xp2p() -> None:
    binary = openwrt_env.REPO_ROOT / "build" / "linux-amd64" / "xp2p"
    skip = os.environ.get("XP2P_OPENWRT_SKIP_HOST_XP2P_BUILD", "").strip().lower() in {"1", "true", "yes"}
    if skip and binary.exists() and binary.stat().st_size > 0:
        print("TIMING: host linux-amd64 xp2p build skipped (env XP2P_OPENWRT_SKIP_HOST_XP2P_BUILD)")
        return
    build_start = time.perf_counter()
    cmd = [
        "go",
        "run",
        "./go/tools/targets",
        "build",
        "--target",
        "linux-amd64",
        "--base",
        "build",
        "--binary",
        "xp2p",
        "--pkg",
        "./go/cmd/xp2p",
    ]
    try:
        subprocess.run(cmd, cwd=openwrt_env.REPO_ROOT, check=True, text=True, capture_output=True)
    except subprocess.CalledProcessError as exc:
        pytest.fail(f"Failed to build host linux-amd64 xp2p.\nSTDOUT:\n{exc.stdout}\nSTDERR:\n{exc.stderr}")
    elapsed = time.perf_counter() - build_start
    if not binary.exists() or binary.stat().st_size == 0:
        pytest.fail(f"Expected linux-amd64 xp2p at {binary} after build.")
    print(f"TIMING: host linux-amd64 xp2p build: {elapsed:.2f}s ({binary})")


def _xp2p_runner(host: Host):
    def _runner(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner


@pytest.fixture(scope="session", autouse=True)
def xp2p_full_cleanup(openwrt_host_factory):
    hosts = [openwrt_host_factory(machine) for machine in openwrt_env.OPENWRT_MACHINES]

    def _cleanup_host(host: Host) -> None:
        runner = _xp2p_runner(host)
        runner("client", "service", "stop")
        runner("server", "service", "stop")
        runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--all",
            "--ignore-missing",
            "--quiet",
        )
        runner(
            "server",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--ignore-missing",
            "--quiet",
        )
        helpers.cleanup_runtime_artifacts(host)
        helpers.reset_install_root_preserve_bin(host)
        helpers._clear_log_root(host)

    for host in hosts:
        _cleanup_host(host)
    yield
    for host in hosts:
        _cleanup_host(host)
