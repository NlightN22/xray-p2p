import time

import pytest
from testinfra.host import Host


def ensure_admin_token(host: Host) -> None:
    from . import env as _env

    last_error: Exception | None = None
    for attempt in range(1, 4):
        marker_local, marker_guest = _env._admin_token_marker()
        if marker_local.exists():
            marker_local.unlink(missing_ok=True)
        try:
            result = _env.run_guest_script(
                host,
                "scripts/ensure_admin_token.ps1",
                MarkerPath=str(marker_guest),
            )
            if result.rc != 0:
                raise RuntimeError(
                    "Failed to ensure admin token.\n"
                    f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
                )
            if not marker_local.exists():
                probe = _env.run_powershell(
                    host,
                    f"if (Test-Path {_env.ps_quote(str(marker_guest))}) {{ exit 0 }} else {{ exit 3 }}",
                )
                if probe.rc != 0:
                    raise RuntimeError("Admin token marker was not created on the guest.")
                marker_local.write_text("OK", encoding="ascii")
            return
        except pytest.skip.Exception as exc:
            last_error = exc
            backend = getattr(host, "backend", None)
            if backend is not None and hasattr(backend, "_reset_client"):
                backend._reset_client()
            if attempt < 3:
                print(f"WARNING: ensure_admin_token retry {attempt} after SSH error: {exc}")
                time.sleep(5)
                continue
            raise
        finally:
            if marker_local.exists():
                marker_local.unlink(missing_ok=True)
            _env.run_powershell(
                host,
                f"if (Test-Path {_env.ps_quote(str(marker_guest))}) {{ Remove-Item -Force {_env.ps_quote(str(marker_guest))} }}",
            )
    if last_error is not None:
        raise last_error

