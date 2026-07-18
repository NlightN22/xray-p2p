package server

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

type serverInstallBase struct {
	installDir  string
	configDir   string
	host        string
	portStr     string
	portVal     int
	certSource  string
	certPath    string
	keyPath     string
	selfSigned  bool
	certStore   string
	installOpts InstallOptions
}

func buildServerInstallBase(installDir, configDir string, opts InstallOptions) (serverInstallBase, error) {
	host := strings.TrimSpace(opts.Host)
	if host == "" {
		return serverInstallBase{}, errors.New("host is required")
	}
	if err := validateCertificateHost(host); err != nil {
		return serverInstallBase{}, err
	}

	portStr := strings.TrimSpace(opts.Port)
	if portStr == "" {
		portStr = strconv.Itoa(DefaultTrojanPort)
	}
	portVal, err := strconv.Atoi(portStr)
	if err != nil || portVal <= 0 || portVal > 65535 {
		return serverInstallBase{}, fmt.Errorf("invalid port %q", portStr)
	}
	profile, err := normalizeInstallProfile(opts.Profile)
	if err != nil {
		return serverInstallBase{}, err
	}

	inputs, err := resolveCertificateInputs(opts.CertificateStore, opts.CertificateFile, opts.KeyFile, opts.RelaxedPathValidation)
	if err != nil {
		return serverInstallBase{}, err
	}

	tunEnabled := opts.TunEnabled
	if !opts.TunEnabledSet {
		tunEnabled = true
	}
	tunName := strings.TrimSpace(opts.TunName)
	if tunName == "" {
		tunName = "xp2ps"
	}
	tunMTU := opts.TunMTU
	if tunMTU <= 0 {
		tunMTU = 1500
	}
	tunAddr := strings.TrimSpace(opts.TunAddr)
	if tunAddr == "" {
		tunAddr = "198.18.0.5/30"
	}

	return serverInstallBase{
		installDir: installDir,
		configDir:  configDir,
		host:       host,
		portStr:    portStr,
		portVal:    portVal,
		certSource: inputs.source,
		certPath:   inputs.certPath,
		keyPath:    inputs.keyPath,
		selfSigned: inputs.selfSigned,
		certStore:  strings.TrimSpace(opts.CertificateStore),
		installOpts: InstallOptions{
			InstallDir:            installDir,
			ConfigDir:             opts.ConfigDir,
			Port:                  portStr,
			CertificateStore:      strings.TrimSpace(opts.CertificateStore),
			CertificateFile:       inputs.certPath,
			KeyFile:               inputs.keyPath,
			Host:                  host,
			Profile:               profile,
			Force:                 opts.Force,
			RelaxedPathValidation: opts.RelaxedPathValidation,
			TunEnabled:            tunEnabled,
			TunEnabledSet:         opts.TunEnabledSet,
			TunName:               tunName,
			TunMTU:                tunMTU,
			TunAddr:               tunAddr,
		},
	}, nil
}

func normalizeInstallProfile(value string) (string, error) {
	endpoint, err := tunnel.DefaultProfile(tunnel.Profile(strings.TrimSpace(value)))
	if err != nil {
		return "", fmt.Errorf("invalid server profile: %w", err)
	}
	return string(endpoint.Profile), nil
}
