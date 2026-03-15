//go:build !windows || !cgo

package uifyne

import "errors"

func Run(_ Options) error {
	return errors.New("fyne backend is unavailable (requires windows + cgo)")
}
