include $(TOPDIR)/rules.mk

PKG_NAME:=xp2p
PKG_VERSION:=0.1.4
PKG_RELEASE:=1
PKG_LICENSE:=MIT
PKG_MAINTAINER:=xray-p2p maintainers
PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)
PKG_BUILD_PARALLEL:=1

include $(INCLUDE_DIR)/package.mk

# Path to the prebuilt xp2p binary that should be packaged.
# Example: make package/xp2p/compile XP2P_PREBUILT=/tmp/build/linux-amd64/xp2p
XP2P_PREBUILT?=/tmp/build/linux-amd64/xp2p

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
	mkdir -p $(PKG_BUILD_DIR)
	cp "$(XP2P_PREBUILT)" $(PKG_BUILD_DIR)/xp2p
	chmod 0755 $(PKG_BUILD_DIR)/xp2p
endef

define Build/Compile
endef

define Package/xp2p/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/xp2p $(1)/usr/bin/xp2p
endef

$(eval $(call BuildPackage,xp2p))
