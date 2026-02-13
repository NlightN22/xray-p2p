#!/bin/sh

XP2P_BIN="/usr/bin/xp2p"
XP2P_INSTALL_ROOT="/etc/xp2p"
XP2P_LOG_ROOT="/var/log/xp2p"

xp2p_prepare_layout() {
	local role="$1"
	[ -n "$role" ] || role="service"
	mkdir -p "$XP2P_INSTALL_ROOT" >/dev/null 2>&1 || true
	mkdir -p "$XP2P_LOG_ROOT/$role" >/dev/null 2>&1 || true
}

xp2p_append_watch() {
	local target="$1"
	[ -z "$target" ] && return 0
	[ -e "$target" ] || return 0
	procd_append_param file "$target"
}

xp2p_watch_config() {
	local role="$1"
	local config_dir="$2"
	[ -n "$role" ] || role="service"
	[ -n "$config_dir" ] || config_dir="config-${role}"
	xp2p_append_watch "$XP2P_INSTALL_ROOT/xp2p-${role}.toml"
	xp2p_append_watch "$XP2P_INSTALL_ROOT/$config_dir/routing.json"
}

xp2p_start_service() {
	local role="$1"
	local config_dir="$2"
	if [ ! -x "$XP2P_BIN" ]; then
		echo "[xp2p] binary $XP2P_BIN not found" >&2
		return 1
	fi
	[ -n "$config_dir" ] || config_dir="config-${role}"
	xp2p_prepare_layout "$role"

	procd_open_instance "$role"
	procd_set_param command "$XP2P_BIN" "$role" service run
	procd_set_param stdout 1
	procd_set_param stderr 1
	procd_set_param respawn 3600 5 5
	xp2p_append_watch "$XP2P_INSTALL_ROOT"
	xp2p_append_watch "$XP2P_INSTALL_ROOT/$config_dir"
	xp2p_watch_config "$role" "$config_dir"
	procd_close_instance
}
