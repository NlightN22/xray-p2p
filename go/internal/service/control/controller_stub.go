//go:build !linux

package control

import "context"

type unsupportedController struct{}

func defaultController() Controller {
	return unsupportedController{}
}

func (unsupportedController) Start(context.Context, Role) error {
	return ErrUnsupported
}

func (unsupportedController) Stop(context.Context, Role) error {
	return ErrUnsupported
}

func (unsupportedController) Status(context.Context, Role) (Status, error) {
	return Status{}, ErrUnsupported
}
