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

	pendingDir, err := pendingConfigDir(state.configDir)
	if err != nil {
		return err
	}
	configPath := filepath.Join(pendingDir, "inbounds.json")
	livePath := filepath.Join(state.configDir, "inbounds.json")
	contents, err := readConfigWithFallback(configPath, livePath)
	if err != nil {
		return err
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

	if state.generateSelfSigned {
		if err := os.MkdirAll(pendingDir, 0o755); err != nil {
			return fmt.Errorf("xp2p: create pending config directory: %w", err)
		}
		state.certDest = filepath.Join(pendingDir, "cert.pem")
		state.keyDest = filepath.Join(pendingDir, "key.pem")
		state.certPath = filepath.Join(state.liveConfigDir, "cert.pem")
		state.keyPath = filepath.Join(state.liveConfigDir, "key.pem")
	} else if state.certSource == CertificateSourcePath {
		if err := os.MkdirAll(pendingDir, 0o755); err != nil {
			return fmt.Errorf("xp2p: create pending config directory: %w", err)
		}
		state.certDest = filepath.Join(pendingDir, "cert.pem")
		state.keyDest = filepath.Join(pendingDir, "key.pem")
		if err := copyFile(state.certPath, state.certDest, 0o644); err != nil {
			return fmt.Errorf("xp2p: copy certificate: %w", err)
		}
		if err := copyFile(state.keyPath, state.keyDest, 0o600); err != nil {
			return fmt.Errorf("xp2p: copy key: %w", err)
		}
		state.certPath = filepath.Join(state.liveConfigDir, "cert.pem")
		state.keyPath = filepath.Join(state.liveConfigDir, "key.pem")
	}
	if err := provisionCertificateFiles(state); err != nil {
		return err
	}

	updateStreamSettings(streamSettings, state)
	trojan["streamSettings"] = streamSettings

	if err := writeInbounds(configPath, root); err != nil {
		return err
	}
	if err := writeServerApplyRequest(); err != nil {
		return err
	}

	logFields := []any{
		"config_dir", state.configDir,
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
	liveConfigDir, err := config.LiveConfigDir(configDir)
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
		certPath = filepath.Join(liveConfigDir, "cert.pem")
		keyPath = filepath.Join(liveConfigDir, "key.pem")
	}

	return certificateState{
		configDir:          configDir,
		liveConfigDir:      liveConfigDir,
		certDest:           filepath.Join(configDir, "cert.pem"),
		keyDest:            filepath.Join(configDir, "key.pem"),
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
	if state.generateSelfSigned {
		logging.Info("xp2p server cert set generating self-signed certificate",
			"host", state.host,
			"destination", state.certDest,
			"valid_years", 10,
		)
		return generateSelfSignedCertificate(state.host, state.certDest, state.keyDest)
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
