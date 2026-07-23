include $(TOPDIR)/rules.mk

PKG_NAME:=xp2p
PKG_VERSION:=0.2.7
PKG_RELEASE:=1
PKG_LICENSE:=MIT
PKG_MAINTAINER:=xray-p2p maintainers
PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)
PKG_BUILD_PARALLEL:=1

include $(INCLUDE_DIR)/package.mk

# Paths to the prebuilt xp2p/xray binaries and completion scripts.
# Example: make package/xp2p/compile XP2P_PREBUILT=/tmp/build/linux-amd64/xp2p XP2P_BUNDLE_DIR=/tmp/build/linux-amd64 XP2P_COMPLETIONS=/tmp/build/linux-amd64/completions
XP2P_PREBUILT?=/tmp/build/linux-amd64/xp2p
XP2P_XRAY?=/tmp/build/linux-amd64/xray
XP2P_COMPLETIONS?=/tmp/build/linux-amd64/completions
XP2P_BUNDLE_DIR?=

define Package/xp2p
  SECTION:=net
  CATEGORY:=Network
  TITLE:=xp2p CLI
  DEPENDS:=+libc
endef

define Package/xp2p/description
Minimal xp2p CLI packaging helper that reuses a prebuilt binary.
Build the binary ahead of time and point XP2P_PREBUILT to it when invoking make.
endef

define Build/Prepare
	$(call Build/Prepare/Default)
	[ -n "$(XP2P_PREBUILT)" ] || { echo "ERROR: XP2P_PREBUILT is not set" >&2; exit 1; }
	[ -f "$(XP2P_PREBUILT)" ] || { echo "ERROR: prebuilt binary $(XP2P_PREBUILT) not found" >&2; exit 1; }
	if [ -n "$(XP2P_BUNDLE_DIR)" ]; then \
		[ -d "$(XP2P_BUNDLE_DIR)" ] || { echo "ERROR: bundle directory $(XP2P_BUNDLE_DIR) not found" >&2; exit 1; }; \
		[ -f "$(XP2P_BUNDLE_DIR)/xray" ] || { echo "ERROR: bundled xray $(XP2P_BUNDLE_DIR)/xray not found" >&2; exit 1; }; \
	else \
		[ -n "$(XP2P_XRAY)" ] || { echo "ERROR: XP2P_XRAY is not set" >&2; exit 1; }; \
		[ -f "$(XP2P_XRAY)" ] || { echo "ERROR: bundled xray $(XP2P_XRAY) not found" >&2; exit 1; }; \
	fi
	[ -n "$(XP2P_COMPLETIONS)" ] || { echo "ERROR: XP2P_COMPLETIONS is not set" >&2; exit 1; }
	[ -f "$(XP2P_COMPLETIONS)/bash/xp2p" ] || { echo "ERROR: bash completion not found under $(XP2P_COMPLETIONS)/bash/xp2p" >&2; exit 1; }
	[ -f "$(XP2P_COMPLETIONS)/zsh/_xp2p" ] || { echo "ERROR: zsh completion not found under $(XP2P_COMPLETIONS)/zsh/_xp2p" >&2; exit 1; }
	[ -f "$(XP2P_COMPLETIONS)/fish/xp2p.fish" ] || { echo "ERROR: fish completion not found under $(XP2P_COMPLETIONS)/fish/xp2p.fish" >&2; exit 1; }
	mkdir -p $(PKG_BUILD_DIR)
	cp "$(XP2P_PREBUILT)" $(PKG_BUILD_DIR)/xp2p
	if [ -n "$(XP2P_BUNDLE_DIR)" ]; then \
		cp -R "$(XP2P_BUNDLE_DIR)/." $(PKG_BUILD_DIR)/; \
		rm -f $(PKG_BUILD_DIR)/.gitkeep; \
	else \
		cp "$(XP2P_XRAY)" $(PKG_BUILD_DIR)/xray; \
	fi
	if [ ! -f "$(PKG_BUILD_DIR)/xray" ] && [ -n "$(XP2P_XRAY)" ] && [ -f "$(XP2P_XRAY)" ]; then \
		cp "$(XP2P_XRAY)" $(PKG_BUILD_DIR)/xray; \
	fi
	[ -f "$(PKG_BUILD_DIR)/xray" ] || { echo "ERROR: bundled xray missing in build directory" >&2; exit 1; }
	mkdir -p $(PKG_BUILD_DIR)/completions
	cp "$(XP2P_COMPLETIONS)/bash/xp2p" $(PKG_BUILD_DIR)/completions/xp2p.bash
	cp "$(XP2P_COMPLETIONS)/zsh/_xp2p" $(PKG_BUILD_DIR)/completions/_xp2p
	cp "$(XP2P_COMPLETIONS)/fish/xp2p.fish" $(PKG_BUILD_DIR)/completions/xp2p.fish
	chmod 0755 $(PKG_BUILD_DIR)/xp2p
	[ -f $(PKG_BUILD_DIR)/xray ] && chmod 0755 $(PKG_BUILD_DIR)/xray || true
	find $(PKG_BUILD_DIR) -maxdepth 1 -type f ! -name 'xray' ! -name 'xp2p' ! -name '.*' -exec chmod 0644 {} +
endef

define Build/Compile
endef

define Package/xp2p/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_DIR) $(1)/etc/xp2p/bin
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_DIR) $(1)/usr/share/bash-completion/completions
	$(INSTALL_DIR) $(1)/usr/share/zsh/vendor-completions
	$(INSTALL_DIR) $(1)/usr/share/fish/vendor_completions.d
	$(INSTALL_DIR) $(1)/usr/lib/xp2p
	$(INSTALL_DIR) $(1)/var/log/xp2p
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/xp2p $(1)/usr/bin/xp2p
	if [ -f $(PKG_BUILD_DIR)/xray ]; then \
		$(INSTALL_BIN) $(PKG_BUILD_DIR)/xray $(1)/etc/xp2p/bin/xray; \
	fi
	find $(PKG_BUILD_DIR) -maxdepth 1 -type f ! -name 'xp2p' ! -name 'xray' ! -name '.*' -exec $(INSTALL_DATA) {} $(1)/etc/xp2p/bin/ \;
	$(INSTALL_BIN) ./files/etc/init.d/xp2p-client $(1)/etc/init.d/xp2p-client
	$(INSTALL_BIN) ./files/etc/init.d/xp2p-server $(1)/etc/init.d/xp2p-server
	$(INSTALL_DATA) ./files/usr/lib/xp2p/procd-service.sh $(1)/usr/lib/xp2p/procd-service.sh
	$(INSTALL_DATA) $(PKG_BUILD_DIR)/completions/xp2p.bash $(1)/usr/share/bash-completion/completions/xp2p
	$(INSTALL_DATA) $(PKG_BUILD_DIR)/completions/_xp2p $(1)/usr/share/zsh/vendor-completions/_xp2p
	$(INSTALL_DATA) $(PKG_BUILD_DIR)/completions/xp2p.fish $(1)/usr/share/fish/vendor_completions.d/xp2p.fish
endef

define Package/xp2p/preinst
#!/bin/sh
if [ -n "$$IPKG_INSTROOT" ] || [ ! -d /etc/xp2p ]; then
	exit 0
fi

archive_root=/etc/xp2p/upgrade-archives
archive_stamp="$$(date -u +%Y%m%dT%H%M%SZ)"
archive_path="$$archive_root/state-$$archive_stamp.tar.gz"
archive_stage="/tmp/xp2p-upgrade-$$"
mkdir -p "$$archive_root" "$$archive_stage/.state" || exit 1
for state_file in /etc/xp2p/state-heartbeat-client.json* /etc/xp2p/state-heartbeat-server.json*; do
	[ -e "$$state_file" ] || continue
	cp -p "$$state_file" "$$archive_stage/" || { rm -rf "$$archive_stage"; exit 1; }
done
for marker in /etc/xp2p/.state/apply.*; do
	[ -e "$$marker" ] || continue
	cp -p "$$marker" "$$archive_stage/.state/" || { rm -rf "$$archive_stage"; exit 1; }
done
for state_dir in live lkg; do
	if [ -d "/etc/xp2p/.state/$$state_dir" ]; then
		cp -R -p "/etc/xp2p/.state/$$state_dir" "$$archive_stage/.state/" || { rm -rf "$$archive_stage"; exit 1; }
	fi
done
tar -czf "$$archive_path" -C "$$archive_stage" . || { rm -rf "$$archive_stage"; exit 1; }
rm -rf "$$archive_stage"
echo "xp2p upgrade state archive: $$archive_path"
exit 0
endef

define Package/xp2p/postinst
#!/bin/sh
if [ -n "$$IPKG_INSTROOT" ]; then
	exit 0
fi

mkdir -p /etc/xp2p
mkdir -p /etc/xp2p/bin
mkdir -p /etc/xp2p/config-client
mkdir -p /etc/xp2p/config-server
mkdir -p /etc/xp2p/.state
mkdir -p /etc/xp2p/.state/live
mkdir -p /etc/xp2p/.state/lkg
mkdir -p /var/log/xp2p
chmod 0755 /etc/xp2p /etc/xp2p/bin /etc/xp2p/config-client /etc/xp2p/config-server /etc/xp2p/.state /etc/xp2p/.state/live /etc/xp2p/.state/lkg /var/log/xp2p
exit 0
endef

define Package/xp2p/prerm
#!/bin/sh
if [ -n "$$IPKG_INSTROOT" ]; then
	exit 0
fi

case "$$1" in
	remove|deconfigure)
		if command -v xp2p >/dev/null 2>&1; then
			mode_output="$$(xp2p client mode 2>/dev/null || true)"
			echo "$$mode_output" | grep -qi "mode: tun" || true
			if echo "$$mode_output" | grep -qi "mode: tun"; then
				if echo "$$mode_output" | grep -qi "tun_mode: full"; then
					xp2p client mode tun split -q >/dev/null 2>&1 || true
				fi
			fi
		fi
		/etc/init.d/xp2p-client stop >/dev/null 2>&1 || true
		/etc/init.d/xp2p-server stop >/dev/null 2>&1 || true
		;;
	upgrade)
		if /etc/init.d/xp2p-client running >/dev/null 2>&1; then
			/etc/init.d/xp2p-client stop >/dev/null 2>&1 || exit 1
		fi
		if /etc/init.d/xp2p-server running >/dev/null 2>&1; then
			/etc/init.d/xp2p-server stop >/dev/null 2>&1 || exit 1
		fi
		;;
esac
exit 0
endef

define Package/xp2p/postrm
#!/bin/sh
if [ -n "$$IPKG_INSTROOT" ]; then
	exit 0
fi

case "$$1" in
	remove)
		/etc/init.d/xp2p-client disable >/dev/null 2>&1 || true
		/etc/init.d/xp2p-server disable >/dev/null 2>&1 || true
		rm -f /etc/xp2p/bin/xray
		;;
esac

purge_requested=0
if [ "$$1" = "purge" ] || [ "$${IPKG_FORCE_REMOVE:-0}" != "0" ] || [ "$${OPKG_FORCE_REMOVE:-0}" != "0" ]; then
	purge_requested=1

else
	parent_pid="$$(awk '{print $$4}' "/proc/$$$$/stat" 2>/dev/null || true)"
	if [ -n "$$parent_pid" ] && [ -r "/proc/$$parent_pid/cmdline" ] && \
		tr '\0' '\n' <"/proc/$$parent_pid/cmdline" | grep -q -- '--force-removal-of-dependent-packages'; then
		purge_requested=1
	fi
fi

if [ "$$purge_requested" -eq 1 ]; then
	rm -rf /etc/xp2p /var/log/xp2p
	rm -f /etc/init.d/xp2p-client /etc/init.d/xp2p-server
fi
exit 0
endef

$(eval $(call BuildPackage,xp2p))
