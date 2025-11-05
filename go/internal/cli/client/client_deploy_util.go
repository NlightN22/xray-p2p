package clientcmd

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

type sshPrerequisites struct {
	sshPath string
	scpPath string
}

func ensureSSHPrerequisites() (sshPrerequisites, error) {
	sshPath, err := resolveExecutable("ssh", "ssh.exe")
	if err != nil {
		return sshPrerequisites{}, fmt.Errorf("ssh client binary not found in PATH (install OpenSSH client and ensure `ssh` is available): %w", err)
	}
	scpPath, err := resolveExecutable("scp", "scp.exe")
	if err != nil {
		return sshPrerequisites{}, fmt.Errorf("scp binary not found in PATH (install OpenSSH client and ensure `scp` is available): %w", err)
	}
	return sshPrerequisites{
		sshPath: sshPath,
		scpPath: scpPath,
	}, nil
}

func resolveExecutable(candidates ...string) (string, error) {
	for _, name := range candidates {
		if path, err := lookPathFunc(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("executables %v not found", candidates)
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
