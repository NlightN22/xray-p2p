package clientcmd

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identity"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func normalizeServerPort(cfg config.Config, flagPort string) string {
	if strings.TrimSpace(flagPort) != "" {
		return strings.TrimSpace(flagPort)
	}
	if cfgPort := strings.TrimSpace(cfg.Client.ServerPort); cfgPort != "" {
		return cfgPort
	}
	if cfgPort := strings.TrimSpace(cfg.Server.TrojanPort); cfgPort != "" {
		return cfgPort
	}
	return fmt.Sprintf("%d", server.DefaultTrojanPort)
}

func generateSecret(size int) (string, error) {
	return identity.NewSecret(size)
}
