package xrayconfig

func DefaultClientConfig() ClientXrayConfig {
	udp := true
	followRedirect := true
	return ClientXrayConfig{
		Inbounds: ClientInboundsConfig{
			Socks: SocksInboundConfig{
				Tag:      "socks-in",
				Protocol: "socks",
				Listen:   "127.0.0.1",
				Port:     51180,
				UDP:      &udp,
			},
			Dokodemo: DokodemoInboundConfig{
				Tag:            "in-48054",
				Remark:         "local-input",
				Protocol:       "dokodemo-door",
				Listen:         "0.0.0.0",
				Port:           48054,
				Network:        "tcp",
				FollowRedirect: &followRedirect,
			},
			Tun: TunInboundConfig{
				Tag:      "tun-in",
				Protocol: "tun",
				Port:     0,
			},
		},
		Logs: defaultLogsConfig("127.0.0.1:52180"),
		Routing: RoutingConfig{
			DomainStrategy: "IPOnDemand",
			Rules:          []map[string]any{},
		},
		DirectOutbound: DirectOutboundConfig{
			Tag:            "direct",
			Protocol:       "freedom",
			DomainStrategy: "UseIP",
		},
	}
}

func DefaultServerConfig() ServerXrayConfig {
	udp := true
	followRedirect := true
	return ServerXrayConfig{
		Inbounds: ServerInboundsConfig{
			Socks: SocksInboundConfig{
				Tag:      "socks-in",
				Protocol: "socks",
				Listen:   "127.0.0.1",
				Port:     51080,
				UDP:      &udp,
			},
			Dokodemo: DokodemoInboundConfig{
				Tag:            "in-48044",
				Remark:         "local-input",
				Protocol:       "dokodemo-door",
				Listen:         "0.0.0.0",
				Port:           48044,
				Network:        "tcp",
				FollowRedirect: &followRedirect,
			},
			Tun: TunInboundConfig{
				Tag:      "tun-in",
				Protocol: "tun",
				Port:     0,
			},
			Trojan: TrojanInboundConfig{
				Tag:           "trojan-in",
				Listen:        "0.0.0.0",
				Protocol:      "trojan",
				Network:       "tcp",
				Security:      "tls",
				AllowInsecure: false,
				Header: TCPHeader{
					Type: "none",
				},
			},
		},
		Logs: defaultLogsConfig("127.0.0.1:52080"),
		Routing: RoutingConfig{
			DomainStrategy: "AsIs",
			Rules:          []map[string]any{},
		},
		DirectOutbound: DirectOutboundConfig{
			Tag:            "direct",
			Protocol:       "freedom",
			DomainStrategy: "UseIP",
		},
	}
}

func defaultLogsConfig(apiListen string) LogsConfig {
	statsEnabled := true
	policyEnabled := true
	return LogsConfig{
		Level:  "warning",
		Access: "none",
		API: APIConfig{
			Tag:      "api",
			Listen:   apiListen,
			Services: []string{"HandlerService", "RoutingService", "StatsService", "LoggerService", "ReflectionService", "ObservatoryService"},
		},
		StatsEnabled: &statsEnabled,
		Policy: PolicyConfig{
			Levels: map[string]PolicyLevel{
				"0": {
					StatsUserDownlink: &policyEnabled,
					StatsUserUplink:   &policyEnabled,
					StatsUserOnline:   &policyEnabled,
				},
			},
			System: PolicySystem{
				StatsInboundDownlink:  &policyEnabled,
				StatsInboundUplink:    &policyEnabled,
				StatsOutboundDownlink: &policyEnabled,
				StatsOutboundUplink:   &policyEnabled,
			},
		},
	}
}
