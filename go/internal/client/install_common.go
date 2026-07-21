package client

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
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
		return clientInstallBase{}, errors.New("client server address is required")
	}

	portStr := strings.TrimSpace(opts.ServerPort)
	if portStr == "" {
		portStr = "8443"
	}
	portVal, err := strconv.Atoi(portStr)
	if err != nil || portVal <= 0 || portVal > 65535 {
		return clientInstallBase{}, fmt.Errorf("invalid client server port %q", portStr)
	}

	password := strings.TrimSpace(opts.Password)
	if password == "" {
		return clientInstallBase{}, errors.New("client password is required")
	}

	user := strings.TrimSpace(opts.User)
	if user == "" {
		return clientInstallBase{}, errors.New("client user email is required")
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
	tunMode := normalizeTunModeValue(opts.TunMode)
	profile, err := normalizeInstallProfile(opts)
	if err != nil {
		return clientInstallBase{}, err
	}

	return clientInstallBase{
		installDir:       installDir,
		configDir:        configDir,
		address:          address,
		portStr:          portStr,
		portVal:          portVal,
		password:         password,
		user:             user,
		serverName:       serverName,
		configFile:       filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName)),
		appliedStateFile: filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName)),
		installOpts: InstallOptions{
			InstallDir:            installDir,
			ConfigDir:             opts.ConfigDir,
			ServerAddress:         address,
			ServerPort:            portStr,
			User:                  user,
			Password:              password,
			ServerName:            serverName,
			ALPN:                  normalizeALPN(opts.ALPN),
			AllowInsecure:         opts.AllowInsecure,
			PinnedPeerCertSHA256:  strings.TrimSpace(opts.PinnedPeerCertSHA256),
			VerifyPeerCertByName:  strings.TrimSpace(opts.VerifyPeerCertByName),
			Profile:               profile.Profile,
			Protocol:              profile.Protocol,
			Transport:             profile.Transport,
			Security:              profile.Security,
			Flow:                  profile.Flow,
			HeartbeatMode:         strings.TrimSpace(opts.HeartbeatMode),
			AllowInsecureOverride: opts.AllowInsecureOverride,
			Force:                 opts.Force,
			TunEnabled:            tunEnabled,
			TunEnabledSet:         opts.TunEnabledSet,
			TunName:               tunName,
			TunMTU:                tunMTU,
			TunAddr:               tunAddr,
			TunMode:               tunMode,
			TunModeSet:            opts.TunModeSet,
		},
	}, nil
}

func normalizeInstallProfile(opts InstallOptions) (installProfile, error) {
	endpoint, err := tunnel.DefaultProfile(tunnel.Profile(strings.TrimSpace(opts.Profile)))
	if err != nil {
		return installProfile{}, fmt.Errorf("invalid client profile: %w", err)
	}
	profile := installProfile{
		Profile:   string(endpoint.Profile),
		Protocol:  endpoint.Protocol,
		Transport: endpoint.Transport,
		Security:  endpoint.Security,
	}
	if endpoint.Metadata != nil {
		profile.Flow = strings.TrimSpace(endpoint.Metadata["flow"])
	}
	if value := strings.TrimSpace(opts.Protocol); value != "" {
		profile.Protocol = value
	}
	if value := strings.TrimSpace(opts.Transport); value != "" {
		profile.Transport = value
	}
	if value := strings.TrimSpace(opts.Security); value != "" {
		profile.Security = value
	}
	if value := strings.TrimSpace(opts.Flow); value != "" {
		profile.Flow = value
	}
	normalized, err := tunnel.Normalize(tunnel.Endpoint{
		Profile:   tunnel.Profile(profile.Profile),
		Protocol:  profile.Protocol,
		Transport: profile.Transport,
		Security:  profile.Security,
		Metadata:  map[string]string{"flow": profile.Flow},
	})
	if err != nil {
		return installProfile{}, fmt.Errorf("invalid client profile: %w", err)
	}
	if normalized.Profile == tunnel.ProfileVLESSTLSVision {
		if err := tunnel.ValidateVLESSCredential(opts.Password); err != nil {
			return installProfile{}, err
		}
	}
	return installProfile{
		Profile:   string(normalized.Profile),
		Protocol:  normalized.Protocol,
		Transport: normalized.Transport,
		Security:  normalized.Security,
		Flow:      strings.TrimSpace(normalized.Metadata["flow"]),
	}, nil
}

type installProfile struct {
	Profile   string
	Protocol  string
	Transport string
	Security  string
	Flow      string
}

func normalizeTunModeValue(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "full":
		return "full"
	default:
		return "split"
	}
}

func ensureClientTunConfig(force bool, tunEnabled bool, tunName string, tunMTU int, tunAddr string, tunMode string, tunModeSet bool) error {
	if _, err := config.EnsureTunSettings("", "client", tunEnabled, tunName, tunMTU, tunAddr); err != nil {
		if force && errors.Is(err, config.ErrConfigParse) {
			configPath := config.PendingConfigPath(layout.ClientConfigFileName)
			if removeErr := os.Remove(configPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removeErr
			}
			if _, retryErr := config.EnsureTunSettings("", "client", tunEnabled, tunName, tunMTU, tunAddr); retryErr != nil {
				return retryErr
			}
		} else {
			return err
		}
	}

	if tunModeSet {
		if _, err := config.UpdateTunMode("", "client", tunMode); err != nil {
			return err
		}
		return nil
	}
	if _, err := config.EnsureTunMode("", "client", tunMode); err != nil {
		return err
	}
	return nil
}
