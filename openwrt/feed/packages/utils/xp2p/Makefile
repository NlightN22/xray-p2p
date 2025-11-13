include $(TOPDIR)/rules.mk

PKG_NAME:=xp2p
PKG_VERSION:=0.1.3
PKG_RELEASE:=1

PKG_SOURCE_PROTO:=git
PKG_SOURCE_URL:=https://github.com/NlightN22/xray-p2p.git
PKG_SOURCE_VERSION:=main
PKG_MIRROR_HASH:=skip

PKG_LICENSE:=MIT
PKG_MAINTAINER:=xp2p Maintainers <maintainers@xp2p.dev>

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

GO_PKG:=github.com/NlightN22/xray-p2p/go/cmd/xp2p
GO_PKG_BUILD_PKG:=$(GO_PKG)
GO_PKG_ENABLE_CGO:=0
GO_PKG_BUILD_VARS+=CGO_ENABLED=0

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/xp2p
  SECTION:=net
  CATEGORY:=Network
  TITLE:=xp2p tunnel helper
  URL:=https://github.com/NlightN22/xray-p2p
DEPENDS:=$(GO_ARCH_DEPENDS) +xray-core
endef

define Package/xp2p/description
xp2p is a cross-platform helper that provisions and runs xray-core-based
client/server tunnels. This package installs the xp2p CLI so that OpenWrt
targets can participate in xp2p-controlled deployments.
endef

define Package/xp2p/install
	$(INSTALL_DIR) $(1)/usr/sbin
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/xp2p $(1)/usr/sbin/xp2p
endef

$(eval $(call GoBinPackage,xp2p))
$(eval $(call BuildPackage,xp2p))
