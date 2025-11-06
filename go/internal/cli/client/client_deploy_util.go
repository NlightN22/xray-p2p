package clientcmd

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/base32"
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

func generateDeployToken() (string, error) {
    // 10 bytes -> 16 base32 chars; lower-case, no padding
    buf := make([]byte, 10)
    if _, err := rand.Read(buf); err != nil {
        return "", err
    }
    enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
    return strings.ToLower(enc), nil
}
