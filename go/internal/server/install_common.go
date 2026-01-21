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
	certPath    string
	keyPath     string
	selfSigned  bool
	certStore   string
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

	inputs, err := resolveCertificateInputs(opts.CertificateStore, opts.CertificateFile, opts.KeyFile, opts.RelaxedPathValidation)
	if err != nil {
		return serverInstallBase{}, err
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
			InstallDir:      installDir,
			ConfigDir:       opts.ConfigDir,
			Port:            portStr,
			CertificateStore: strings.TrimSpace(opts.CertificateStore),
			CertificateFile: inputs.certPath,
			KeyFile:         inputs.keyPath,
			Host:            host,
			Force:           opts.Force,
			RelaxedPathValidation: opts.RelaxedPathValidation,
		},
	}, nil
}
