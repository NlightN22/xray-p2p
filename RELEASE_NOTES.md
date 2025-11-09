# Release notes

## Windows MSI deliverable

- Every tagged release (`release.yml`) now spawns a dedicated Windows job that installs the WiX Toolset, runs `go run ./go/tools/targets build --target windows-amd64 ...`, and compiles `installer/wix/xp2p.wxs` into `xp2p-<version>-windows-amd64.msi`.
- The MSI enters the GitHub Release assets alongside the existing platform archives and is also copied to a rolling `xp2p-latest-windows-amd64.msi` artifact.
- Custom properties:
  - `XP2P_CLIENT_ARGS` – triggers `xp2p client install <args>` after files deploy.
  - `XP2P_SERVER_ARGS` – triggers `xp2p server install <args>` after the client action completes.
  - `INSTALLFOLDER` – overrides the default `C:\Program Files\xp2p`.
  - `MSIINSTALLPERUSER=1` – forces a per-user `%LOCALAPPDATA%\xp2p` install even when elevation is available.

## Local validation workflow

1. Install WiX locally (Chocolatey or the official MSI) and ensure `candle`/`light` are reachable.
2. Build the Windows binary: `go run ./go/tools/targets build --target windows-amd64 --base build --binary xp2p --pkg ./go/cmd/xp2p --ldflags "-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$(go run ./go/cmd/xp2p --version)"`.
3. Generate the installer:
   ```powershell
   candle -dProductVersion=<version> -dXp2pBinary=build/windows-amd64/xp2p.exe installer/wix/xp2p.wxs
   light -out dist/xp2p-<version>-windows-amd64.msi installer/wix/xp2p.wixobj
   ```
4. Test the installer modes:
   - Per-machine (silent): `msiexec /i dist/xp2p-<version>-windows-amd64.msi /qn`.
   - Per-user fallback: run the same command from a limited user or append `MSIINSTALLPERUSER=1`.
   - Custom CLI bootstrap: append `XP2P_CLIENT_ARGS="--link trojan://..."` or `XP2P_SERVER_ARGS="--deploy-file ..."`.
5. Validate uninstall and repair flows: `msiexec /x dist/xp2p-<version>-windows-amd64.msi /qn` and `msiexec /fa dist/xp2p-<version>-windows-amd64.msi`.
