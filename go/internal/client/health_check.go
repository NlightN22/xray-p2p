package client

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

const (
	socksHealthTimeout  = 30 * time.Second
	socksHealthInterval = 500 * time.Millisecond
)

func resolveClientSocksAddress(configFile string) (string, error) {
	defaults := xrayconfig.DefaultClientConfig().Inbounds.Socks
	cfg, err := xrayconfig.LoadClientConfig(configFile)
	if err != nil {
		return socksAddress(defaults.Listen, defaults.Port, defaults.Listen, defaults.Port), err
	}
	socks := cfg.Inbounds.Socks
	return socksAddress(socks.Listen, socks.Port, defaults.Listen, defaults.Port), nil
}

func socksAddress(listen string, port int, fallbackListen string, fallbackPort int) string {
	host := strings.TrimSpace(listen)
	if host == "" {
		host = strings.TrimSpace(fallbackListen)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = fallbackPort
	}
	if port <= 0 {
		port = 51180
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}
