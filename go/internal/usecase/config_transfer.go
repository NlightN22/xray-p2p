package usecase

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/configbundle"
)

type ConfigTransfer struct{}

func NewConfigTransfer() *ConfigTransfer {
	return &ConfigTransfer{}
}

func (c *ConfigTransfer) Export(ctx context.Context, root, outputPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return configbundle.ExportConfigRoot(root, outputPath)
}

func (c *ConfigTransfer) Import(ctx context.Context, root, inputPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return configbundle.ImportConfigRoot(root, inputPath)
}
