package preflight

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
)

type TunPreflight interface {
	Check(ctx context.Context, cfg TunConfig) error
}

type TunConfig struct {
	Enabled        bool
	Name           string
	Addr           string
	MTU            int
	Mode           string
	NonInteractive bool
	WintunDLLPath  string
}

type ErrTunUnavailable struct {
	OS     string
	Reason string
	Hint   string
}

func (e ErrTunUnavailable) Error() string {
	osName := strings.TrimSpace(e.OS)
	if osName == "" {
		osName = runtime.GOOS
	}
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = "unknown reason"
	}
	hint := strings.TrimSpace(e.Hint)
	if hint == "" {
		return fmt.Sprintf("tun is unavailable on %s: %s", osName, reason)
	}
	return fmt.Sprintf("tun is unavailable on %s: %s (hint: %s)", osName, reason, hint)
}

func IsTunUnavailable(err error) bool {
	var tunErr ErrTunUnavailable
	return errors.As(err, &tunErr)
}

func CheckTun(ctx context.Context, cfg TunConfig) error {
	if !cfg.Enabled {
		return nil
	}
	return Tun().Check(ctx, cfg)
}
