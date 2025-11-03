//go:build windows

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	configPath := filepath.Join(state.configDir, "inbounds.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("xp2p: read %s: %w", configPath, err)
	}

	root, err := parseInbounds(contents)
	if err != nil {
		return err
	}

	trojan, err := selectTrojanInbound(root)
	if err != nil {
		return err
	}

	streamSettings, err := extractStreamSettings(trojan)
	if err != nil {
		return err
	}

	if hasTLSConfigured(streamSettings) && !state.force {
		return ErrCertificateConfigured
	}

	if err := provisionCertificateFiles(state); err != nil {
		return err
	}

	updateStreamSettings(streamSettings, state)
	trojan["streamSettings"] = streamSettings

	if err := writeInbounds(configPath, root); err != nil {
		return err
	}

	logFields := []any{
		"config_dir", state.configDir,
		"cert_path", state.certDest,
	}
	if state.sourceCertificate != "" {
		logging.Info("xp2p server cert set deployed provided certificate", logFields...)
	} else {
		logging.Info("xp2p server cert set generated self-signed certificate",
			append(logFields, "host", state.host, "valid_years", 10)...,
		)
	}
	return nil
}

type certificateState struct {
	configDir          string
	certDest           string
	keyDest            string
	sourceCertificate  string
	sourceKey          string
	host               string
	force              bool
	generateSelfSigned bool
}

var errHostRequired = errors.New("xp2p: host required")

func normalizeCertificateOptions(opts CertificateOptions) (certificateState, error) {
	if strings.TrimSpace(opts.InstallDir) == "" && !filepath.IsAbs(strings.TrimSpace(opts.ConfigDir)) {
		return certificateState{}, errors.New("xp2p: install directory is required when config dir is relative")
	}

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

	certSource := strings.TrimSpace(opts.CertificateFile)
	keySource := strings.TrimSpace(opts.KeyFile)
	host := strings.TrimSpace(opts.Host)

	if certSource == "" && keySource != "" {
		return certificateState{}, errors.New("xp2p: key file provided without certificate file")
	}

	if certSource != "" {
		if err := ensureFileExists(certSource); err != nil {
			return certificateState{}, fmt.Errorf("xp2p: certificate: %w", err)
		}
		if keySource != "" {
			if err := ensureFileExists(keySource); err != nil {
				return certificateState{}, fmt.Errorf("xp2p: key: %w", err)
			}
		}
	} else {
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

	return certificateState{
		configDir:          configDir,
		certDest:           filepath.Join(configDir, "cert.pem"),
		keyDest:            filepath.Join(configDir, "key.pem"),
		sourceCertificate:  certSource,
		sourceKey:          keySource,
		host:               host,
		force:              opts.Force,
		generateSelfSigned: certSource == "",
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
	if installDir == "" {
		return "", errors.New("xp2p: install directory is required when config dir is relative")
	}
	return filepath.Join(installDir, cfg), nil
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

func extractStreamSettings(trojan map[string]any) (map[string]any, error) {
	rawSettings, ok := trojan["streamSettings"]
	if !ok {
		settings := make(map[string]any)
		trojan["streamSettings"] = settings
		return settings, nil
	}
	settings, ok := rawSettings.(map[string]any)
	if !ok {
		return nil, errors.New("xp2p: trojan streamSettings invalid")
	}
	return settings, nil
}

func hasTLSConfigured(stream map[string]any) bool {
	value, _ := stream["security"].(string)
	return strings.EqualFold(strings.TrimSpace(value), "tls")
}

func provisionCertificateFiles(state certificateState) error {
	if state.generateSelfSigned {
		logging.Info("xp2p server cert set generating self-signed certificate",
			"host", state.host,
			"destination", state.certDest,
			"valid_years", 10,
		)
		return generateSelfSignedCertificate(state.host, state.certDest, state.keyDest)
	}

	mode := os.FileMode(0o644)
	if err := copyFile(state.sourceCertificate, state.certDest, mode); err != nil {
		return fmt.Errorf("xp2p: copy certificate: %w", err)
	}

	keySource := state.sourceKey
	if keySource == "" {
		keySource = state.sourceCertificate
	}
	if err := copyFile(keySource, state.keyDest, 0o600); err != nil {
		return fmt.Errorf("xp2p: copy key: %w", err)
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
		"certificateFile": filepath.ToSlash(state.certDest),
		"keyFile":         filepath.ToSlash(state.keyDest),
	}
	tlsSettings["certificates"] = []any{certEntry}
	stream["tlsSettings"] = tlsSettings
}
