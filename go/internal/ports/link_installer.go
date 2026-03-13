package ports

import "context"

type LinkInstaller interface {
	Install(ctx context.Context, link string) error
}
