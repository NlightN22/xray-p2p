//go:build !windows

package control

import "context"

func SetServiceEnv(_ context.Context, _ Role, _ map[string]string) error {
	return ErrUnsupported
}
