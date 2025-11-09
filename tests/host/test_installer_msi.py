from __future__ import annotations

import textwrap
from pathlib import Path

import pytest

from tests.host import _env

REPO_ROOT = Path(r"C:\xp2p")
MSI_BUILD_DIR = REPO_ROOT / "build" / "msi-test"
MSI_MIN_SIZE_BYTES = 1_000_000
MARKER_PATH = "__MSI__"
MARKER_SIZE = "__MSI_SIZE__"


def _ps_path(path: Path) -> str:
    return str(path).replace("\\", "\\\\")


def _extract_marker(output: str, marker: str) -> str | None:
    for line in (output or "").splitlines():
        stripped = line.strip()
        if stripped.startswith(marker):
            return stripped.split("=", 1)[1].strip()
    return None


@pytest.mark.host
def test_windows_installer_builds_msi(server_host):
    repo_ps = _ps_path(REPO_ROOT)
    msi_dir_ps = _ps_path(MSI_BUILD_DIR)
    script = textwrap.dedent(
        f"""
        $ErrorActionPreference = 'Stop'
        $repo = '{repo_ps}'
        $msiOutDir = '{msi_dir_ps}'
        if (Test-Path $msiOutDir) {{
            Remove-Item $msiOutDir -Recurse -Force -ErrorAction SilentlyContinue
        }}
        New-Item -ItemType Directory -Path $msiOutDir -Force | Out-Null
        Push-Location $repo
        try {{
            $version = (& go run ./go/cmd/xp2p --version).Trim()
            if ([string]::IsNullOrWhiteSpace($version)) {{
                throw "xp2p --version returned empty output."
            }}
            $ldflags = "-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$version"
            $binaryDir = Join-Path $repo 'build\\msi-bin'
            if (Test-Path $binaryDir) {{
                Remove-Item $binaryDir -Recurse -Force -ErrorAction SilentlyContinue
            }}
            New-Item -ItemType Directory -Path $binaryDir -Force | Out-Null
            $binaryPath = Join-Path $binaryDir 'xp2p.exe'
            & go build -trimpath -ldflags $ldflags -o $binaryPath ./go/cmd/xp2p
            if ($LASTEXITCODE -ne 0) {{
                throw "go build failed with exit code $LASTEXITCODE"
            }}
            if (-not (Test-Path $binaryPath)) {{
                throw "xp2p binary missing at $binaryPath"
            }}
            $wixDir = Get-ChildItem "C:\\Program Files (x86)" -Filter "WiX Toolset*" -Directory |
                Sort-Object LastWriteTime -Descending |
                Select-Object -First 1
            if (-not $wixDir) {{
                throw "WiX Toolset installation directory not found."
            }}
            $candle = Join-Path $wixDir.FullName 'bin\\candle.exe'
            $light = Join-Path $wixDir.FullName 'bin\\light.exe'
            if (-not (Test-Path $candle)) {{
                throw "candle.exe missing at $candle"
            }}
            if (-not (Test-Path $light)) {{
                throw "light.exe missing at $light"
            }}
            $wixSource = Join-Path $repo 'installer\\wix\\xp2p.wxs'
            $wixObj = Join-Path $msiOutDir 'xp2p.wixobj'
            & $candle "-dProductVersion=$version" "-dXp2pBinary=$binaryPath" "-out" $wixObj $wixSource
            if ($LASTEXITCODE -ne 0) {{
                throw "candle.exe failed with exit code $LASTEXITCODE"
            }}
            $msiPath = Join-Path $msiOutDir ("xp2p-$version-windows-amd64.msi")
            & $light "-out" $msiPath $wixObj
            if ($LASTEXITCODE -ne 0) {{
                throw "light.exe failed with exit code $LASTEXITCODE"
            }}
            $msiItem = Get-Item $msiPath -ErrorAction Stop
            $attempt = 0
            while ($msiItem.Length -le 0 -and $attempt -lt 10) {{
                Start-Sleep -Milliseconds 200
                $msiItem = Get-Item $msiPath -ErrorAction Stop
                $attempt++
            }}
                Write-Output ("{MARKER_PATH}={{0}}" -f $msiItem.FullName)
                Write-Output ("{MARKER_SIZE}={{0}}" -f $msiItem.Length)
        }}
        finally {{
            Pop-Location
        }}
        """
    ).strip()

    result = _env.run_powershell(server_host, script)
    stdout = result.stdout or ""
    if result.rc != 0:
        pytest.fail(
            "Failed to build MSI via WiX.\n"
            f"STDOUT:\n{stdout}\nSTDERR:\n{result.stderr}"
        )

    msi_path = _extract_marker(stdout, MARKER_PATH)
    assert msi_path, f"MSI path marker missing in output:\n{stdout}"

    size_raw = _extract_marker(stdout, MARKER_SIZE)
    assert size_raw, f"MSI size marker missing in output:\n{stdout}"
    try:
        size_value = int(size_raw)
    except ValueError as exc:
        pytest.fail(f"Invalid MSI size marker value {size_raw!r}: {exc}")

    assert size_value >= MSI_MIN_SIZE_BYTES, (
        f"Expected MSI to be at least {MSI_MIN_SIZE_BYTES} bytes, got {size_value}"
    )
