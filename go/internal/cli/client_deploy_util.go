package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func ensureSSHPrerequisites() error {
	if _, err := lookPathFunc("ssh"); err != nil {
		return fmt.Errorf("ssh binary not found in PATH")
	}
	if _, err := lookPathFunc("scp"); err != nil {
		return fmt.Errorf("scp binary not found in PATH")
	}
	return nil
}

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

func promptString(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}
