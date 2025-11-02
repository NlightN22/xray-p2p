package xray

import _ "embed"

// Version is the bundled xray-core release identifier.
const Version = "25.10.15"

//go:embed bin/25.10.15/win-amd64/xray.exe
var windowsAMD64 []byte

// WindowsAMD64 returns the embedded xray-core binary for Windows on amd64.
func WindowsAMD64() []byte {
	return windowsAMD64
}
