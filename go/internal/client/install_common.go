package client

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type clientInstallBase struct {
	installDir       string
	configDir        string
	address          string
	portStr          string
	portVal          int
	password         string
	user             string
	serverName       string
	configFile       string
	appliedStateFile string
	installOpts      InstallOptions
}

func buildClientInstallBase(installDir, configDir string, opts InstallOptions) (clientInstallBase, error) {
	address := strings.TrimSpace(opts.ServerAddress)
	if address == "" {
		return clientInstallBase{}, errors.New("xp2p: client server address is required")
	}

	portStr := strings.TrimSpace(opts.ServerPort)
	if portStr == "" {
		portStr = "8443"
	}
	portVal, err := strconv.Atoi(portStr)
	if err != nil || portVal <= 0 || portVal > 65535 {
		return clientInstallBase{}, fmt.Errorf("xp2p: invalid client server port %q", portStr)
	}

	password := strings.TrimSpace(opts.Password)
	if password == "" {
		return clientInstallBase{}, errors.New("xp2p: client password is required")
	}

	user := strings.TrimSpace(opts.User)
	if user == "" {
		return clientInstallBase{}, errors.New("xp2p: client user email is required")
	}

	serverName := strings.TrimSpace(opts.ServerName)
	if serverName == "" {
		serverName = address
	}

	tunEnabled := opts.TunEnabled
	if !opts.TunEnabledSet {
		tunEnabled = true
	}
	tunName := strings.TrimSpace(opts.TunName)
	if tunName == "" {
		tunName = "xp2pc"
	}
	tunMTU := opts.TunMTU
	if tunMTU <= 0 {
		tunMTU = 1500
	}
	tunAddr := strings.TrimSpace(opts.TunAddr)
	if tunAddr == "" {
		tunAddr = "198.18.0.1/30"
	}

	return clientInstallBase{
		installDir: installDir,
		configDir:  configDir,
		address:    address,
		portStr:    portStr,
		portVal:    portVal,
		password:   password,
		user:       user,
		serverName: serverName,
		configFile: filepath.Clean(layout.ClientConfigFileName),
		appliedStateFile: filepath.Clean(layout.ClientAppliedStateFileName),
		installOpts: InstallOptions{
			InstallDir:            installDir,
			ConfigDir:             opts.ConfigDir,
			ServerAddress:         address,
			ServerPort:            portStr,
			User:                  user,
			Password:              password,
			ServerName:            serverName,
			AllowInsecure:         opts.AllowInsecure,
			AllowInsecureOverride: opts.AllowInsecureOverride,
			Force:                 opts.Force,
			TunEnabled:            tunEnabled,
			TunEnabledSet:         opts.TunEnabledSet,
			TunName:               tunName,
			TunMTU:                tunMTU,
			TunAddr:               tunAddr,
		},
	}, nil
}
