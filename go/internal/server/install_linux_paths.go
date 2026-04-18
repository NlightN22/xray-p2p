//go:build linux

package server

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
)

func normalizeInstallOptions(opts InstallOptions) (installState, error) {
	dir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return installState{}, err
	}

	configDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return installState{}, err
	}
	base, err := buildServerInstallBase(dir, configDir, opts)
	if err != nil {
		return installState{}, err
	}

	logsDir := filepath.Join(config.LogRoot(), "server")

	state := installState{
		InstallOptions: base.installOpts,
		installDir:     base.installDir,
		configDir:      base.configDir,
		logsDir:        logsDir,
		portValue:      base.portVal,
		selfSigned:     base.selfSigned,
		stateFile:      filepath.Join(dir, installstate.FileNameForKind(installstate.KindServer)),
		certSource:     base.certSource,
	}

	return state, nil
}

func resolveInstallDir(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", errors.New("install directory is required")
	}
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("resolve install directory: %w", err)
		}
		cleaned = abs
	}
	return cleaned, nil
}

func ResolveConfigDir(base, cfg string) (string, error) {
	cfg = strings.TrimSpace(cfg)
	if cfg == "" {
		cfg = DefaultServerConfigDir
	}
	if filepath.IsAbs(cfg) {
		return cfg, nil
	}
	return filepath.Join(base, cfg), nil
}
