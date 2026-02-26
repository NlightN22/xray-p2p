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
	certDest           string
	keyDest            string
	certPath           string
	keyPath            string
	host               string
	force              bool
	generateSelfSigned bool
	certSource         string
	allowInsecure      bool
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
		certPath = filepath.Join(configDir, "cert.pem")
		keyPath = filepath.Join(configDir, "key.pem")
	}

	allowInsecure := inputs.selfSigned
	if !inputs.selfSigned && inputs.source == CertificateSourcePath {
		selfSigned, err := isSelfSignedCertificatePath(inputs.certPath)
		if err != nil {
			return certificateState{}, err
		}
		allowInsecure = selfSigned
	}

	return certificateState{
		configDir:          configDir,
		certDest:           filepath.Join(configDir, "cert.pem"),
		keyDest:            filepath.Join(configDir, "key.pem"),
		certPath:           certPath,
		keyPath:            keyPath,
		host:               host,
		force:              opts.Force,
		generateSelfSigned: inputs.selfSigned,
		certSource:         inputs.source,
		allowInsecure:      allowInsecure,
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
	if state.allowInsecure {
		tlsSettings["allowInsecure"] = true
	} else {
		delete(tlsSettings, "allowInsecure")
	}
	stream["tlsSettings"] = tlsSettings
}
