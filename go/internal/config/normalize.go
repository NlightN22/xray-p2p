package config

import "strings"

func normalize(cfg *Config) {
	cfg.Logging.Level = strings.TrimSpace(strings.ToLower(cfg.Logging.Level))
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = defaultValues["logging.level"].(string)
	}
	cfg.Logging.Format = strings.TrimSpace(strings.ToLower(cfg.Logging.Format))
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = defaultValues["logging.format"].(string)
	}

	cfg.Server.Port = strings.TrimSpace(cfg.Server.Port)
	if cfg.Server.Port == "" {
		cfg.Server.Port = defaultValues["server.port"].(string)
	}
	cfg.Server.TrojanPort = strings.TrimSpace(cfg.Server.TrojanPort)
	if cfg.Server.TrojanPort == "" {
		cfg.Server.TrojanPort = defaultValues["server.trojan_port"].(string)
	}

	cfg.Server.InstallDir = strings.TrimSpace(cfg.Server.InstallDir)
	if cfg.Server.InstallDir == "" {
		cfg.Server.InstallDir = defaultInstallDir()
	}

	cfg.Server.ConfigDir = strings.TrimSpace(cfg.Server.ConfigDir)
	if cfg.Server.ConfigDir == "" {
		cfg.Server.ConfigDir = defaultValues["server.config_dir"].(string)
	}

	cfg.Server.Mode = strings.TrimSpace(strings.ToLower(cfg.Server.Mode))
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = defaultValues["server.mode"].(string)
	}

	cfg.Server.CertificateStore = strings.TrimSpace(cfg.Server.CertificateStore)
	if cfg.Server.CertificateStore == "" {
		cfg.Server.CertificateStore = defaultValues["server.cert_store"].(string)
	}

	cfg.Server.CertificateFile = strings.TrimSpace(cfg.Server.CertificateFile)
	if cfg.Server.CertificateFile == "" {
		cfg.Server.CertificateFile = defaultValues["server.certificate"].(string)
	}

	cfg.Server.KeyFile = strings.TrimSpace(cfg.Server.KeyFile)
	if cfg.Server.KeyFile == "" {
		cfg.Server.KeyFile = defaultValues["server.key"].(string)
	}

	cfg.Server.Host = strings.TrimSpace(cfg.Server.Host)
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaultValues["server.host"].(string)
	}

	cfg.Server.TunName = strings.TrimSpace(cfg.Server.TunName)
	if cfg.Server.TunName == "" {
		cfg.Server.TunName = defaultValues["server.tun_name"].(string)
	}

	if cfg.Server.TunMTU <= 0 {
		cfg.Server.TunMTU = defaultValues["server.tun_mtu"].(int)
	}

	cfg.Server.TunAddr = strings.TrimSpace(cfg.Server.TunAddr)
	if cfg.Server.TunAddr == "" {
		cfg.Server.TunAddr = defaultValues["server.tun_addr"].(string)
	}

	cfg.Client.InstallDir = strings.TrimSpace(cfg.Client.InstallDir)
	if cfg.Client.InstallDir == "" {
		cfg.Client.InstallDir = defaultInstallDir()
	}

	cfg.Client.ConfigDir = strings.TrimSpace(cfg.Client.ConfigDir)
	if cfg.Client.ConfigDir == "" {
		cfg.Client.ConfigDir = defaultValues["client.config_dir"].(string)
	}

	cfg.Client.ServerAddress = strings.TrimSpace(cfg.Client.ServerAddress)
	if cfg.Client.ServerAddress == "" {
		cfg.Client.ServerAddress = defaultValues["client.server_address"].(string)
	}

	cfg.Client.ServerPort = strings.TrimSpace(cfg.Client.ServerPort)
	if cfg.Client.ServerPort == "" {
		cfg.Client.ServerPort = defaultValues["client.server_port"].(string)
	}

	cfg.Client.DiagPort = strings.TrimSpace(cfg.Client.DiagPort)
	if cfg.Client.DiagPort == "" {
		cfg.Client.DiagPort = defaultValues["client.diag_port"].(string)
	}

	cfg.Client.User = strings.TrimSpace(cfg.Client.User)
	if cfg.Client.User == "" {
		cfg.Client.User = defaultValues["client.user"].(string)
	}

	cfg.Client.Password = strings.TrimSpace(cfg.Client.Password)
	if cfg.Client.Password == "" {
		cfg.Client.Password = defaultValues["client.password"].(string)
	}

	cfg.Client.ServerName = strings.TrimSpace(cfg.Client.ServerName)
	if cfg.Client.ServerName == "" {
		cfg.Client.ServerName = defaultValues["client.server_name"].(string)
	}

	cfg.Client.SocksAddress = strings.TrimSpace(cfg.Client.SocksAddress)
	if cfg.Client.SocksAddress == "" {
		cfg.Client.SocksAddress = defaultValues["client.socks_address"].(string)
	}

	cfg.Client.TunName = strings.TrimSpace(cfg.Client.TunName)
	if cfg.Client.TunName == "" {
		cfg.Client.TunName = defaultValues["client.tun_name"].(string)
	}

	if cfg.Client.TunMTU <= 0 {
		cfg.Client.TunMTU = defaultValues["client.tun_mtu"].(int)
	}

	cfg.Client.TunAddr = strings.TrimSpace(cfg.Client.TunAddr)
	if cfg.Client.TunAddr == "" {
		cfg.Client.TunAddr = defaultValues["client.tun_addr"].(string)
	}
	cfg.Client.TunMode = normalizeTunMode(cfg.Client.TunMode)
	cfg.Client.DNSServers = normalizeDNSServers(cfg.Client.DNSServers)
	cfg.Client.FullTunnelTag = strings.TrimSpace(cfg.Client.FullTunnelTag)

	// AllowInsecure is a boolean and defaults through the map loader.
}

func normalizeTunMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "full":
		return "full"
	case "split":
		return "split"
	default:
		return defaultValues["client.tun_mode"].(string)
	}
}

func normalizeDNSServers(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		trimmed = append(trimmed, clean)
	}
	if len(trimmed) == 0 {
		return []string{}
	}
	return trimmed
}
