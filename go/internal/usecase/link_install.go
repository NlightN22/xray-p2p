package usecase

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/ports"
)

type LinkInstall struct {
	installer ports.LinkInstaller
}

func NewLinkInstall(installer ports.LinkInstaller) *LinkInstall {
	return &LinkInstall{installer: installer}
}

func (l *LinkInstall) Install(ctx context.Context, link string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return l.installer.Install(ctx, link)
}
