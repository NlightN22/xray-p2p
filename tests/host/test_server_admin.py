import pytest


@pytest.mark.host
def test_server_has_admin_rights(server_host):
    script = (
        "[Security.Principal.WindowsPrincipal]::new("
        "[Security.Principal.WindowsIdentity]::GetCurrent()"
        ").IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)"
    )
    result = server_host.run(f'powershell -NoProfile -Command "{script}"')
    assert result.rc == 0
    assert result.stdout.strip().lower() == "true"
