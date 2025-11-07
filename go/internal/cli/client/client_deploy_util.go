package clientcmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func normalizeServerPort(cfg config.Config, flagPort string) string {
	if strings.TrimSpace(flagPort) != "" {
		return strings.TrimSpace(flagPort)
	}
	if cfgPort := strings.TrimSpace(cfg.Server.Port); cfgPort != "" && cfgPort != server.DefaultPort {
		return cfgPort
	}
	return fmt.Sprintf("%d", server.DefaultTrojanPort)
}

func generateSecret(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
