package ui

import (
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/gen2brain/beeep"
)

func Notify(title, message string) {
	if err := beeep.Notify(title, message, ""); err != nil {
		logging.Warn("xp2p-ui notification failed", "err", err)
	}
}
