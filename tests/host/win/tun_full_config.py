from __future__ import annotations

from pathlib import Path

from tests.host.win import env as _env
from tests.host.win import tun_full_helpers as helpers


def client_mode(runner, *args: str, check: bool = True):
    return runner(
        "client",
        "mode",
        *args,
        "--path",
        str(helpers.CLIENT_INSTALL_DIR),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        check=check,
    )


def update_client_config(host, **updates) -> None:
    original = _env.read_text(host, helpers.CLIENT_CONFIG_FILE)
    updated = _update_toml_section(original, "client", updates)
    if updated != original:
        write_text_atomic(host, helpers.CLIENT_CONFIG_FILE, updated)


def _update_toml_section(text: str, section: str, updates: dict) -> str:
    lines = text.splitlines()
    out: list[str] = []
    in_section = False
    section_found = False
    seen_keys: set[str] = set()
    section_header = f"[{section}]"

    for line in lines:
        stripped = line.strip()
        if stripped.startswith("[") and stripped.endswith("]") and not stripped.startswith("[["):
            if in_section:
                for key, value in updates.items():
                    if key not in seen_keys:
                        out.append(f"{key} = {_toml_value(value)}")
                        seen_keys.add(key)
                in_section = False
            section_found = section_found or stripped == section_header
            in_section = stripped == section_header
            out.append(line)
            continue
        if in_section and "=" in stripped:
            key = stripped.split("=", 1)[0].strip()
            if key in updates:
                if key not in seen_keys:
                    out.append(f"{key} = {_toml_value(updates[key])}")
                    seen_keys.add(key)
                continue
        out.append(line)

    if in_section:
        for key, value in updates.items():
            if key not in seen_keys:
                out.append(f"{key} = {_toml_value(value)}")
                seen_keys.add(key)

    if not section_found:
        if out and out[-1].strip():
            out.append("")
        out.append(section_header)
        for key, value in updates.items():
            out.append(f"{key} = {_toml_value(value)}")

    return "\n".join(out) + "\n"


def _toml_value(value) -> str:
    if isinstance(value, list):
        entries = ", ".join(f"\"{entry}\"" for entry in value)
        return f"[{entries}]"
    if isinstance(value, str):
        return f"\"{value}\""
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    raise TypeError(f"Unsupported TOML value: {value!r}")


def write_text_atomic(host, path: Path, content: str) -> None:
    encoded = _env.ps_quote(content.encode("utf-8").hex())
    target = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
$hex = {encoded}
$bytes = for ($i = 0; $i -lt $hex.Length; $i += 2) {{
    [Convert]::ToByte($hex.Substring($i, 2), 16)
}}
$text = [System.Text.Encoding]::UTF8.GetString($bytes)
$dir = Split-Path -Parent $target
if ($dir -and -not (Test-Path $dir)) {{
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}}
$tempName = [System.IO.Path]::GetRandomFileName()
$tempPath = Join-Path $dir $tempName
$encoding = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($tempPath, $text, $encoding)
Move-Item -Path $tempPath -Destination $target -Force
"""
    result = _env.run_powershell(host, script, label="write_text_atomic")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
