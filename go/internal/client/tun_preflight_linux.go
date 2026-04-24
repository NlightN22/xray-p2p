//go:build linux

package client

import (
	"fmt"
	"os"
)

func PreflightTunDevice() error {
	const path = "/dev/net/tun"
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("TUN device is required for tun mode but %s is missing", path)
		}
		return fmt.Errorf("TUN device is not accessible at %s: %w", path, err)
	}
	_ = f.Close()
	return nil
}

