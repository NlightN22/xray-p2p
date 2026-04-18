package xrayconfig

import (
	"errors"
	"fmt"
	"strings"
)

func validateClientConfig(cfg ClientXrayConfig) error {
	if err := validateSocks(cfg.Inbounds.Socks, "client"); err != nil {
		return err
	}
	if err := validateDokodemo(cfg.Inbounds.Dokodemo, "client"); err != nil {
		return err
	}
	if err := validateTun(cfg.Inbounds.Tun, "client"); err != nil {
		return err
	}
	if err := validateLogs(cfg.Logs, "client"); err != nil {
		return err
	}
	if err := validateRouting(cfg.Routing, "client"); err != nil {
		return err
	}
	if err := validateDirectOutbound(cfg.DirectOutbound, "client"); err != nil {
		return err
	}
	return nil
}

func validateServerConfig(cfg ServerXrayConfig) error {
	if err := validateSocks(cfg.Inbounds.Socks, "server"); err != nil {
		return err
	}
	if err := validateDokodemo(cfg.Inbounds.Dokodemo, "server"); err != nil {
		return err
	}
	if err := validateTun(cfg.Inbounds.Tun, "server"); err != nil {
		return err
	}
	if err := validateTrojan(cfg.Inbounds.Trojan); err != nil {
		return err
	}
	if err := validateLogs(cfg.Logs, "server"); err != nil {
		return err
	}
	if err := validateRouting(cfg.Routing, "server"); err != nil {
		return err
	}
	if err := validateDirectOutbound(cfg.DirectOutbound, "server"); err != nil {
		return err
	}
	return nil
}

func validateSocks(cfg SocksInboundConfig, scope string) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return fmt.Errorf("xrayconfig: %s socks protocol is required", scope)
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		return fmt.Errorf("xrayconfig: %s socks listen is required", scope)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("xrayconfig: %s socks port is invalid", scope)
	}
	return nil
}

func validateDokodemo(cfg DokodemoInboundConfig, scope string) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return fmt.Errorf("xrayconfig: %s dokodemo protocol is required", scope)
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		return fmt.Errorf("xrayconfig: %s dokodemo listen is required", scope)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("xrayconfig: %s dokodemo port is invalid", scope)
	}
	if strings.TrimSpace(cfg.Network) == "" {
		return fmt.Errorf("xrayconfig: %s dokodemo network is required", scope)
	}
	return nil
}

func validateTun(cfg TunInboundConfig, scope string) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return fmt.Errorf("xrayconfig: %s tun protocol is required", scope)
	}
	if strings.TrimSpace(cfg.Tag) == "" {
		return fmt.Errorf("xrayconfig: %s tun tag is required", scope)
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("xrayconfig: %s tun port is invalid", scope)
	}
	return nil
}

func validateTrojan(cfg TrojanInboundConfig) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return errors.New("xrayconfig: trojan protocol is required")
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		return errors.New("xrayconfig: trojan listen is required")
	}
	if strings.TrimSpace(cfg.Network) == "" {
		return errors.New("xrayconfig: trojan network is required")
	}
	if strings.TrimSpace(cfg.Security) == "" {
		return errors.New("xrayconfig: trojan security is required")
	}
	headerType := strings.TrimSpace(cfg.Header.Type)
	if headerType == "" {
		return errors.New("xrayconfig: trojan header type is required")
	}
	if !strings.EqualFold(headerType, "none") && strings.TrimSpace(cfg.Header.Request.Method) == "" {
		return errors.New("xrayconfig: trojan header request method is required")
	}
	return nil
}

func validateLogs(cfg LogsConfig, scope string) error {
	if strings.TrimSpace(cfg.Level) == "" {
		return fmt.Errorf("xrayconfig: %s logs level is required", scope)
	}
	if strings.TrimSpace(cfg.API.Listen) == "" {
		return fmt.Errorf("xrayconfig: %s logs api listen is required", scope)
	}
	if strings.TrimSpace(cfg.API.Tag) == "" {
		return fmt.Errorf("xrayconfig: %s logs api tag is required", scope)
	}
	return nil
}

func validateRouting(cfg RoutingConfig, scope string) error {
	if strings.TrimSpace(cfg.DomainStrategy) == "" {
		return fmt.Errorf("xrayconfig: %s routing domain_strategy is required", scope)
	}
	return nil
}

func validateDirectOutbound(cfg DirectOutboundConfig, scope string) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return fmt.Errorf("xrayconfig: %s direct outbound protocol is required", scope)
	}
	if strings.TrimSpace(cfg.Tag) == "" {
		return fmt.Errorf("xrayconfig: %s direct outbound tag is required", scope)
	}
	return nil
}
