//go:build !windows

package ui

func EnsureAutoStart(_ bool) error {
	return nil
}
