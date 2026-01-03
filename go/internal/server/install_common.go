package server

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type serverInstallBase struct {
	installDir  string
	configDir   string
	host        string
	portStr     string
	portVal     int
	certSource  string
	keySource   string
	selfSigned  bool
	installOpts InstallOptions
}

func buildServerInstallBase(installDir, configDir string, opts InstallOptions) (serverInstallBase, error) {
	host := strings.TrimSpace(opts.Host)
	if host == "" {
		return serverInstallBase{}, errors.New("xp2p: host is required")
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
		return serverInstallBase{}, fmt.Errorf("xp2p: invalid port %q", portStr)
	}

	certSource := strings.TrimSpace(opts.CertificateFile)
	keySource := strings.TrimSpace(opts.KeyFile)

	if certSource != "" {
		if err := ensureFileExists(certSource); err != nil {
			return serverInstallBase{}, fmt.Errorf("xp2p: certificate: %w", err)
		}
		if keySource != "" {
			if err := ensureFileExists(keySource); err != nil {
				return serverInstallBase{}, fmt.Errorf("xp2p: key: %w", err)
			}
		}
	}

	if certSource == "" && keySource != "" {
		return serverInstallBase{}, errors.New("xp2p: key file provided without certificate file")
	}

	return serverInstallBase{
		installDir: installDir,
		configDir:  configDir,
		host:       host,
		portStr:    portStr,
		portVal:    portVal,
		certSource: certSource,
		keySource:  keySource,
		selfSigned: certSource == "",
		installOpts: InstallOptions{
			InstallDir:      installDir,
			ConfigDir:       opts.ConfigDir,
			Port:            portStr,
			CertificateFile: certSource,
			KeyFile:         keySource,
			Host:            host,
			Force:           opts.Force,
		},
	}, nil
}
