//go:build linux

package control

import "sync"

var (
	controllerOnce sync.Once
	controllerImpl Controller
)

func defaultController() Controller {
	controllerOnce.Do(func() {
		if isOpenWrtSystem() {
			controllerImpl = procdController{}
		} else {
			controllerImpl = systemdController{}
		}
	})
	return controllerImpl
}
