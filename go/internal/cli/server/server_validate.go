package servercmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
)

func determineInstallHost(ctx context.Context, explicit, fallback string) (string, bool, error) {
	host := clishared.FirstNonEmpty(explicit, fallback)
	host = strings.TrimSpace(host)
	if host != "" {
		if err := netutil.ValidateHost(host); err != nil {
			return "", false, fmt.Errorf("invalid host %q: %w", host, err)
		}
		return host, false, nil
	}
	value, err := detectPublicHostFunc(ctx)
	if err != nil {
		return "", false, fmt.Errorf("%w (use --host to specify the public address)", err)
	}
	value = strings.TrimSpace(value)
	if err := netutil.ValidateHost(value); err != nil {
		return "", false, fmt.Errorf("invalid host %q: %w", value, err)
	}
	return value, true, nil
}

func validatePortValue(port string) error {
	value := strings.TrimSpace(port)
	if value == "" {
		return fmt.Errorf("port is empty")
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid port %q: %w", value, err)
	}
	if n <= 0 || n > 65535 {
		return fmt.Errorf("invalid port %q: must be within 1-65535", value)
	}
	return nil
}
