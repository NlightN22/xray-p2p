package main

func applyOverlays(root schema) {
	defs := root["definitions"].(map[string]any)
	setEnum(defs, "config_logging_config", "level", "debug", "info", "warn", "error")
	setEnum(defs, "config_logging_config", "format", "text", "json")
	setEnum(defs, "forward_rule", "protocol", "tcp", "udp", "both")
	setEnum(defs, "redirect_rule", "access", "all", "restricted")
	setEnum(defs, "client_endpoint_group", "mode", "automatic", "manual", "disabled")
	setEnum(defs, "client_client_endpoint_record", "heartbeat_mode", "auto", "required", "disabled")
	setEnum(defs, "tunnel_user", "credential_generation")

	for _, name := range []string{"forward_rule", "ha_member", "xrayconfig_socks_inbound_config", "xrayconfig_dokodemo_inbound_config"} {
		setRange(defs, name, "port", 1, 65535)
	}
	setRange(defs, "client_endpoint", "port", 1, 65535)
	setRange(defs, "forward_rule", "listen_port", 1, 65535)
	setRange(defs, "forward_rule", "target_port", 1, 65535)

	for _, name := range []string{"config_xray_assets_config", "config_xray_asset_config"} {
		setPattern(defs, name, "stale_after", `^[0-9]+(ns|us|ms|s|m|h)$`)
	}
	for _, name := range []string{"config_client_config", "config_server_config"} {
		setRange(defs, name, "tun_mtu", 576, 65535)
	}

	if redirect, ok := defs["redirect_rule"].(schema); ok {
		redirect["oneOf"] = []any{
			schema{"required": []string{"cidr"}, "properties": schema{"domain": schema{"maxLength": 0}}},
			schema{"required": []string{"domain"}, "properties": schema{"cidr": schema{"maxLength": 0}}},
		}
	}
}

func property(defs map[string]any, definition, field string) schema {
	def, ok := defs[definition].(schema)
	if !ok {
		return nil
	}
	props, _ := def["properties"].(map[string]any)
	prop, _ := props[field].(schema)
	return prop
}

func setEnum(defs map[string]any, definition, field string, values ...string) {
	if len(values) == 0 {
		return
	}
	if prop := property(defs, definition, field); prop != nil {
		prop["enum"] = values
	}
}

func setRange(defs map[string]any, definition, field string, minimum, maximum int) {
	if prop := property(defs, definition, field); prop != nil {
		prop["minimum"] = minimum
		prop["maximum"] = maximum
	}
}

func setPattern(defs map[string]any, definition, field, pattern string) {
	if prop := property(defs, definition, field); prop != nil {
		prop["pattern"] = pattern
	}
}
