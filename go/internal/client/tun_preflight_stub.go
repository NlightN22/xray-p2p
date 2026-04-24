//go:build !linux

package client

func PreflightTunDevice() error {
	return nil
}

