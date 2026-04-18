//go:build windows || linux

package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/extensions"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func serverArtifactsPresent(state installState) (bool, string, error) {
	if _, err := installstate.Read(state.stateFile, installstate.KindServer); err == nil {
		return true, fmt.Sprintf("state file %s", state.stateFile), nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, installstate.ErrRoleNotInstalled) {
		return false, "", fmt.Errorf("read server state: %w", err)
	}

	desiredPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if _, err := os.Stat(desiredPath); err == nil {
		return true, fmt.Sprintf("desired config %s", desiredPath), nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, "", fmt.Errorf("stat %s: %w", desiredPath, err)
	}

	liveXray, err := config.LiveXrayPath(apply.RoleServer)
	if err != nil {
		return false, "", err
	}
	if _, err := os.Stat(liveXray); err == nil {
		return true, fmt.Sprintf("live artifact %s", liveXray), nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, "", fmt.Errorf("stat %s: %w", liveXray, err)
	}
	return false, "", nil
}

func deployDesiredConfiguration(state installState) error {
	if _, err := ensureServerXrayConfigForce(pendingConfigPath(), state.Force); err != nil {
		return err
	}
	if err := extensions.EnsureTemplates(state.configDir); err != nil {
		return err
	}
	if state.selfSigned || state.certSource == CertificateSourcePath {
		if err := os.MkdirAll(defaultTLSDir(), 0o755); err != nil {
			return fmt.Errorf("create tls dir: %w", err)
		}
		if state.selfSigned {
			logging.Info("xp2p server install generating self-signed certificate",
				"host", state.Host,
				"valid_years", 10,
				"destination", defaultCertPath(),
			)
			if err := generateSelfSignedCertificate(state.Host, defaultCertPath(), defaultKeyPath()); err != nil {
				return err
			}
		} else {
			if err := copyFile(state.CertificateFile, defaultCertPath(), 0o644); err != nil {
				return fmt.Errorf("copy certificate: %w", err)
			}
			if err := copyFile(state.KeyFile, defaultKeyPath(), 0o600); err != nil {
				return fmt.Errorf("copy key: %w", err)
			}
		}
	}
	return nil
}
