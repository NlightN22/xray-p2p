from __future__ import annotations

import pytest

from tests.host.win import env as win_env
from tests.host.win.flows import apply as apply_flow
from tests.host.win.flows import scm as scm_flow
from tests.host.win.flows import service_cli as svc


@pytest.mark.host
def test_windows_client_service_cli_controls_service(
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    runner = xp2p_client_runner

    with svc.timed("client cleanup (pre)"):
        svc.cleanup_role(
            client_host,
            "client",
            remove_config=True,
            log_paths=[svc.CLIENT_SERVICE_LOG],
        )

    with svc.timed("client install"):
        svc.install_client(runner, "10.70.0.10", "svc-client@example.com", "SvcClientSecret")

    svc.require_service_installed(client_host, "client")

    with svc.timed("client service start"):
        runner("client", "service", "start", check=True)
    svc.wait_for_service_state_cli(runner, "client", expected_active=True)
    svc.wait_for_apply_request_clear(client_host)
    if svc.current_mode(client_host, "client") == "tun":
        svc.assert_ipv6_binding_disabled(client_host, svc.CLIENT_TUN)
    assert win_env.path_exists(client_host, svc.CLIENT_SERVICE_LOG), "client service log not created"

    with svc.timed("client service stop (final)"):
        runner("client", "service", "stop", check=True)
    svc.wait_for_service_state_cli(runner, "client", expected_active=False)


@pytest.mark.host
@pytest.mark.parametrize("role", ["client", "server"])
def test_windows_service_restarts_when_config_changes(
    role,
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    if role == "client":
        host = client_host
        runner = xp2p_client_runner
        log_path = svc.CLIENT_SERVICE_LOG
        original_mode: str | None = None
        install_fn = lambda: svc.install_client(
            runner,
            "10.70.0.20",
            "svc-change@example.com",
            "SvcChangeSecret",
        )

        def change_fn():
            nonlocal original_mode
            original_mode = svc.toggle_mode(host, runner, role)

        def revert_fn():
            if original_mode:
                svc.set_mode(runner, role, original_mode)

    else:
        host = server_host
        runner = xp2p_server_runner
        log_path = svc.SERVER_SERVICE_LOG
        original_mode: str | None = None
        install_fn = lambda: svc.install_server(
            runner,
            "svc-server.example.com",
            "62180",
        )

        def change_fn():
            nonlocal original_mode
            original_mode = svc.toggle_mode(host, runner, role)

        def revert_fn():
            if original_mode:
                svc.set_mode(runner, role, original_mode)

    with svc.timed(f"{role} cleanup (pre)"):
        svc.cleanup_role(
            host,
            role,
            remove_config=True,
            log_paths=[log_path],
        )
    with svc.timed(f"{role} install"):
        install_fn()
    svc.require_service_installed(host, role)
    try:
        with svc.timed(f"{role} clear logs"):
            svc.cleanup_role(
                host,
                role,
                remove_config=False,
                log_paths=[log_path],
            )
        with svc.timed(f"{role} service start"):
            runner(role, "service", "start", check=True)
        svc.wait_for_service_state_cli(runner, role, expected_active=True)
        svc.wait_for_apply_request_clear(host)
        if svc.current_mode(host, role) == "tun":
            if role == "client":
                svc.assert_ipv6_binding_disabled(host, svc.CLIENT_TUN)

        change_applied = False
        try:
            with svc.timed(f"{role} change config"):
                change_fn()
                change_applied = True

            svc.wait_for_log_entry(host, log_path, "service configuration change detected")
            svc.wait_for_apply_request_clear(host)
            svc.wait_for_service_state_cli(runner, role, expected_active=True)
        finally:
            if change_applied:
                with svc.timed(f"{role} revert config"):
                    revert_fn()
    finally:
        with svc.timed(f"{role} cleanup (final)"):
            svc.cleanup_role(
                host,
                role,
                remove_config=True,
                log_paths=None,
            )


@pytest.mark.host
@pytest.mark.parametrize("role", ["client", "server"])
def test_windows_service_records_apply_error_after_invalid_config(
    role,
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    if role == "client":
        host = client_host
        runner = xp2p_client_runner
        log_path = svc.CLIENT_SERVICE_LOG
        config_path = svc.CLIENT_CONFIG_FILE
        install_fn = lambda: svc.install_client(
            runner,
            "10.70.0.30",
            "svc-fail@example.com",
            "SvcFailSecret",
        )
        live_dir = win_env.CLIENT_LIVE_DIR
    else:
        host = server_host
        runner = xp2p_server_runner
        log_path = svc.SERVER_SERVICE_LOG
        config_path = svc.SERVER_CONFIG_FILE
        install_fn = lambda: svc.install_server(
            runner,
            "svc-fail.example.com",
            "62190",
        )
        live_dir = win_env.SERVER_LIVE_DIR

    with svc.timed(f"{role} cleanup (pre)"):
        svc.cleanup_role(
            host,
            role,
            remove_config=True,
            log_paths=[log_path],
        )
    with svc.timed(f"{role} install"):
        install_fn()
    svc.require_service_installed(host, role)
    try:
        with svc.timed(f"{role} clear logs"):
            svc.cleanup_role(
                host,
                role,
                remove_config=False,
                log_paths=[log_path],
            )
        with svc.timed(f"{role} service start"):
            runner(role, "service", "start", check=True)
        svc.wait_for_service_state_cli(runner, role, expected_active=True)
        svc.wait_for_apply_request_clear(host)

        apply_error_path = apply_flow.apply_error_path()
        with svc.timed(f"{role} clear apply.error"):
            win_env.remove_path(host, apply_error_path)
        assert not win_env.path_exists(host, apply_error_path), "apply.error was not cleared"

        live_xray_json = live_dir / "xray.json"
        assert win_env.path_exists(host, live_xray_json), f"Missing live xray config: {live_xray_json}"
        before_live_xray_text = win_env.read_text(host, live_xray_json)
        assert (before_live_xray_text or "").strip(), f"Empty live xray config: {live_xray_json}"

        with svc.timed(f"{role} write broken config"):
            svc.write_text_utf8(host, config_path, "BROKEN-CONFIG")
        with svc.timed(f"{role} service restart"):
            runner(role, "service", "restart")

        apply_flow.wait_for_apply_error_set(
            host,
            timeout=svc.SERVICE_TIMEOUT,
            poll_seconds=svc.POLL_INTERVAL,
            dump_label=f"{role}-apply-error",
        )
        scm_flow.wait_for_service_status(
            host,
            name=f"xp2p-{role}",
            expected="running",
            timeout=svc.SERVICE_TIMEOUT,
            poll_seconds=svc.POLL_INTERVAL,
            dump_label=f"{role}-scm-running",
        )

        apply_error_text = win_env.read_text(host, apply_error_path)
        assert (apply_error_text or "").strip(), "apply.error is empty"
        apply_error_lower = (apply_error_text or "").lower()
        if "parse" not in apply_error_lower and "toml" not in apply_error_lower:
            pytest.fail(f"apply.error does not look like a parse error:\n{apply_error_text}")

        apply_request_path = apply_flow.apply_request_path()
        assert win_env.path_exists(
            host, apply_request_path
        ), "apply.request is expected to remain set after failed apply"

        svc.wait_for_log_entry_any(
            host,
            log_path,
            [
                "apply compilation failed",
                "parse error",
            ],
        )

        after_live_xray_text = win_env.read_text(host, live_xray_json)
        assert after_live_xray_text == before_live_xray_text, "Live xray config changed after failed apply"
    finally:
        with svc.timed(f"{role} cleanup (final)"):
            svc.cleanup_role(
                host,
                role,
                remove_config=True,
                log_paths=None,
            )
