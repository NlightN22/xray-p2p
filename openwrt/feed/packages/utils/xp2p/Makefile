include $(TOPDIR)/rules.mk

PKG_NAME:=xp2p
PKG_VERSION:=0.2.3
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
		for f in "$(XP2P_BUNDLE_DIR)"/*; do \
			base=$$(basename "$$f"); \
			[ "$$base" = ".gitkeep" ] && continue; \
			[ -f "$$f" ] || continue; \
			cp "$$f" $(PKG_BUILD_DIR)/"$$base"; \
		done; \
	else \
		cp "$(XP2P_XRAY)" $(PKG_BUILD_DIR)/xray; \
	fi
	mkdir -p $(PKG_BUILD_DIR)/completions
	cp "$(XP2P_COMPLETIONS)/bash/xp2p" $(PKG_BUILD_DIR)/completions/xp2p.bash
	cp "$(XP2P_COMPLETIONS)/zsh/_xp2p" $(PKG_BUILD_DIR)/completions/_xp2p
	cp "$(XP2P_COMPLETIONS)/fish/xp2p.fish" $(PKG_BUILD_DIR)/completions/xp2p.fish
	chmod 0755 $(PKG_BUILD_DIR)/xp2p
	[ -f $(PKG_BUILD_DIR)/xray ] && chmod 0755 $(PKG_BUILD_DIR)/xray || true
	for f in $(PKG_BUILD_DIR)/*; do \
		[ -f "$$f" ] || continue; \
		base=$$(basename "$$f"); \
		if [ "$$base" = "xray" ] || [ "$$base" = "xp2p" ]; then \
			continue; \
		fi; \
		chmod 0644 "$$f"; \
	done
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
	for f in $(PKG_BUILD_DIR)/*; do \
		[ -f "$$f" ] || continue; \
		base=$$(basename "$$f"); \
		if [ "$$base" = "xp2p" ]; then \
			continue; \
		fi; \
		if [ "$$base" = "xray" ]; then \
			$(INSTALL_BIN) "$$f" $(1)/etc/xp2p/bin/xray; \
			continue; \
		fi; \
		$(INSTALL_DATA) "$$f" $(1)/etc/xp2p/bin/"$$base"; \
	done
	$(INSTALL_BIN) ./files/etc/init.d/xp2p-client $(1)/etc/init.d/xp2p-client
	$(INSTALL_BIN) ./files/etc/init.d/xp2p-server $(1)/etc/init.d/xp2p-server
	$(INSTALL_DATA) ./files/usr/lib/xp2p/procd-service.sh $(1)/usr/lib/xp2p/procd-service.sh
	$(INSTALL_DATA) $(PKG_BUILD_DIR)/completions/xp2p.bash $(1)/usr/share/bash-completion/completions/xp2p
	$(INSTALL_DATA) $(PKG_BUILD_DIR)/completions/_xp2p $(1)/usr/share/zsh/vendor-completions/_xp2p
	$(INSTALL_DATA) $(PKG_BUILD_DIR)/completions/xp2p.fish $(1)/usr/share/fish/vendor_completions.d/xp2p.fish
endef

define Package/xp2p/postinst
#!/bin/sh
if [ -n "$$IPKG_INSTROOT" ]; then
	exit 0
fi

mkdir -p /etc/xp2p
mkdir -p /etc/xp2p/bin
mkdir -p /var/log/xp2p
chmod 0755 /etc/xp2p /etc/xp2p/bin /var/log/xp2p
/etc/init.d/xp2p-client enable >/dev/null 2>&1 || true
/etc/init.d/xp2p-server enable >/dev/null 2>&1 || true
exit 0
endef

define Package/xp2p/prerm
#!/bin/sh
if [ -n "$$IPKG_INSTROOT" ]; then
	exit 0
fi

case "$$1" in
	remove|upgrade|deconfigure)
		/etc/init.d/xp2p-client stop >/dev/null 2>&1 || true
		/etc/init.d/xp2p-server stop >/dev/null 2>&1 || true
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
	remove|upgrade|failed-upgrade|abort-install|abort-upgrade)
		/etc/init.d/xp2p-client disable >/dev/null 2>&1 || true
		/etc/init.d/xp2p-server disable >/dev/null 2>&1 || true
		rm -f /etc/xp2p/bin/xray
		;;
esac

purge_requested=0
if [ "$$1" = "purge" ] || [ "$${IPKG_FORCE_REMOVE:-0}" != "0" ] || [ "$${OPKG_FORCE_REMOVE:-0}" != "0" ]; then
	purge_requested=1
elif [ -n "$$PPID" ] && [ -r "/proc/$$PPID/cmdline" ]; then
	if tr '\0' '\n' <"/proc/$$PPID/cmdline" | grep -q -- '--force-removal-of-dependent-packages'; then
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
