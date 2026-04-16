//go:build windows || linux

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

// SetCertificate provisions TLS material for an existing installation.
func SetCertificate(ctx context.Context, opts CertificateOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	state, err := normalizeCertificateOptions(opts)
	if err != nil {
		if errors.Is(err, errHostRequired) {
			return fmt.Errorf("xp2p: host is required to generate self-signed certificate (use --host or configure server.host)")
		}
		return err
	}

	if err := ensureConfigExists(state.configDir); err != nil {
		return err
	}
	if err := provisionCertificateFiles(state); err != nil {
		return err
	}
	if err := writeServerApplyRequest(); err != nil {
		return err
	}
	logFields := []any{
		"tls_dir", defaultTLSDir(),
		"cert_path", state.certPath,
	}
	if state.generateSelfSigned {
		logging.Info("xp2p server cert set generated self-signed certificate",
			append(logFields, "host", state.host, "valid_years", 10)...,
		)
	} else {
		logging.Info("xp2p server cert set configured certificate paths", logFields...)
	}
	return nil
}

type certificateState struct {
	configDir          string
	liveConfigDir      string
	certDest           string
	keyDest            string
	certPath           string
	keyPath            string
	host               string
	force              bool
	generateSelfSigned bool
	certSource         string
}

var errHostRequired = errors.New("xp2p: host required")

func normalizeCertificateOptions(opts CertificateOptions) (certificateState, error) {
	installDir := opts.InstallDir
	if installDir != "" {
		resolved, err := resolveInstallDir(installDir)
		if err != nil {
			return certificateState{}, err
		}
		installDir = resolved
	}

	configDir, err := resolveCertificateConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return certificateState{}, err
	}

	inputs, err := resolveCertificateInputs(opts.CertificateStore, opts.CertificateFile, opts.KeyFile, opts.RelaxedPathValidation)
	if err != nil {
		return certificateState{}, err
	}
	host := strings.TrimSpace(opts.Host)

	if inputs.selfSigned {
		if host == "" {
			return certificateState{}, errHostRequired
		}
		if err := validateCertificateHost(host); err != nil {
			return certificateState{}, err
		}
	}

	if host != "" {
		if err := validateCertificateHost(host); err != nil {
			return certificateState{}, err
		}
	}

	certPath := inputs.certPath
	keyPath := inputs.keyPath
	if inputs.selfSigned {
		certPath = defaultCertPath()
		keyPath = defaultKeyPath()
	}

	return certificateState{
		configDir:          configDir,
		liveConfigDir:      "",
		certDest:           defaultCertPath(),
		keyDest:            defaultKeyPath(),
		certPath:           certPath,
		keyPath:            keyPath,
		host:               host,
		force:              opts.Force,
		generateSelfSigned: inputs.selfSigned,
		certSource:         inputs.source,
	}, nil
}

func resolveCertificateConfigDir(installDir, configDir string) (string, error) {
	cfg := strings.TrimSpace(configDir)
	if cfg == "" {
		cfg = DefaultServerConfigDir
	}
	if filepath.IsAbs(cfg) {
		return cfg, nil
	}
	return filepath.Join(config.ConfigRoot(), cfg), nil
}

func ensureConfigExists(configDir string) error {
	info, err := os.Stat(configDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: configuration directory %s does not exist (run server install first)", configDir)
		}
		return fmt.Errorf("xp2p: stat %s: %w", configDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("xp2p: %s is not a directory", configDir)
	}
	return nil
}

func hasTLSConfigured(stream map[string]any) bool {
	value, _ := stream["security"].(string)
	return strings.EqualFold(strings.TrimSpace(value), "tls")
}

func provisionCertificateFiles(state certificateState) error {
	if err := os.MkdirAll(filepath.Dir(state.certDest), 0o755); err != nil {
		return fmt.Errorf("xp2p: create tls dir: %w", err)
	}
	if !state.force {
		if _, err := os.Stat(state.certDest); err == nil {
			return ErrCertificateConfigured
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: stat %s: %w", state.certDest, err)
		}
		if _, err := os.Stat(state.keyDest); err == nil {
			return ErrCertificateConfigured
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: stat %s: %w", state.keyDest, err)
		}
	}
	if state.generateSelfSigned {
		logging.Info("xp2p server cert set generating self-signed certificate",
			"host", state.host,
			"destination", state.certDest,
			"valid_years", 10,
		)
		return generateSelfSignedCertificate(state.host, state.certDest, state.keyDest)
	}
	if state.certSource == CertificateSourcePath {
		if err := copyFile(state.certPath, state.certDest, 0o644); err != nil {
			return fmt.Errorf("xp2p: copy certificate: %w", err)
		}
		if err := copyFile(state.keyPath, state.keyDest, 0o600); err != nil {
			return fmt.Errorf("xp2p: copy key: %w", err)
		}
	}
	return nil
}

func updateStreamSettings(stream map[string]any, state certificateState) {
	stream["security"] = "tls"

	tlsSettings, _ := stream["tlsSettings"].(map[string]any)
	if tlsSettings == nil {
		tlsSettings = make(map[string]any)
	}

	certEntry := map[string]any{
		"certificateFile": filepath.ToSlash(state.certPath),
		"keyFile":         filepath.ToSlash(state.keyPath),
	}
	tlsSettings["certificates"] = []any{certEntry}
	stream["tlsSettings"] = tlsSettings
}
