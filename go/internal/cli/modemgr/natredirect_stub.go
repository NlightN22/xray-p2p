//go:build !linux

package modemgr

// ApplyNatRedirectMode is a no-op on non-Linux platforms.
func ApplyNatRedirectMode(string) error {
	return nil
}
