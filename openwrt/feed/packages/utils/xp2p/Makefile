include $(TOPDIR)/rules.mk

PKG_NAME:=xp2p
PKG_VERSION:=0.1.4
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

HOST_GO ?= $(STAGING_DIR_HOSTPKG)/bin/go
COMPLETION_GO_BIN:=$(if $(wildcard $(HOST_GO)),$(HOST_GO),go)
COMPLETIONS_DIR:=$(PKG_BUILD_DIR)/.completions
COMPLETION_BASH:=$(COMPLETIONS_DIR)/xp2p.bash
COMPLETION_ZSH:=$(COMPLETIONS_DIR)/_xp2p
COMPLETION_FISH:=$(COMPLETIONS_DIR)/xp2p.fish

TARGET_GOARCH:=$(strip $(or $(GO_PKG_TARGET_ARCH),$(GO_ARCH),$(ARCH)))

define MapXrayBundleArch
$(strip \
  $(if $(filter amd64 x86_64,$(1)),x86_64, \
    $(if $(filter 386 i386,$(1)),x86, \
      $(if $(filter arm64 aarch64,$(1)),arm64, \
        $(if $(filter arm armhf armv5 armv6 armv7 armv7l,$(1)),arm32, \
          $(if $(filter mipsle mipsel,$(1)),mips32le, \
            $(if $(filter mips64le mips64el,$(1)),mips64le, \
              $(if $(filter riscv64,$(1)),riscv64, \
                $(error Unsupported GO arch '$(1)' for xp2p xray bundle) \
              ) \
            ) \
          ) \
        ) \
      ) \
    ) \
  ) \
)
endef

XRAY_BUNDLE_ARCH:=$(call MapXrayBundleArch,$(TARGET_GOARCH))
XRAY_BUNDLE_PATH:=$(PKG_BUILD_DIR)/distro/linux/bundle/$(XRAY_BUNDLE_ARCH)/xray

define Build/GenerateCompletions
	rm -rf $(COMPLETIONS_DIR)
	mkdir -p $(COMPLETIONS_DIR)
	cd $(PKG_BUILD_DIR); \
		GO111MODULE=on CGO_ENABLED=0 GOOS= GOARCH= "$(COMPLETION_GO_BIN)" run ./go/cmd/xp2p completion bash > $(COMPLETION_BASH); \
		GO111MODULE=on CGO_ENABLED=0 GOOS= GOARCH= "$(COMPLETION_GO_BIN)" run ./go/cmd/xp2p completion zsh > $(COMPLETION_ZSH); \
		GO111MODULE=on CGO_ENABLED=0 GOOS= GOARCH= "$(COMPLETION_GO_BIN)" run ./go/cmd/xp2p completion fish > $(COMPLETION_FISH)
endef

define Build/Compile
	$(call GoPackage/Build/Compile)
	$(call Build/GenerateCompletions)
endef

define Package/xp2p
  SECTION:=net
  CATEGORY:=Network
  TITLE:=xp2p tunnel helper
  URL:=https://github.com/NlightN22/xray-p2p
DEPENDS:=$(GO_ARCH_DEPENDS)
endef

define Package/xp2p/description
xp2p is a cross-platform helper that provisions and runs xray-core-based
client/server tunnels. This package installs the xp2p CLI so that OpenWrt
targets can participate in xp2p-controlled deployments.
endef

define Package/xp2p/install
	$(INSTALL_DIR) \
		$(1)/usr/bin \
		$(1)/etc/xp2p/bin \
		$(1)/etc/xp2p/config-client \
		$(1)/etc/xp2p/config-server \
		$(1)/var/log/xp2p \
		$(1)/usr/share/bash-completion/completions \
		$(1)/usr/share/zsh/vendor-completions \
		$(1)/usr/share/fish/vendor_completions.d
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/xp2p $(1)/usr/bin/xp2p
	@if [ ! -f "$(XRAY_BUNDLE_PATH)" ]; then \
		echo "xray bundle for $(TARGET_GOARCH) not found at $(XRAY_BUNDLE_PATH)" >&2; \
		exit 1; \
	fi
	$(INSTALL_BIN) $(XRAY_BUNDLE_PATH) $(1)/etc/xp2p/bin/xray
	$(INSTALL_DATA) $(COMPLETION_BASH) $(1)/usr/share/bash-completion/completions/xp2p
	$(INSTALL_DATA) $(COMPLETION_ZSH) $(1)/usr/share/zsh/vendor-completions/_xp2p
	$(INSTALL_DATA) $(COMPLETION_FISH) $(1)/usr/share/fish/vendor_completions.d/xp2p.fish
endef

$(eval $(call GoBinPackage,xp2p))
$(eval $(call BuildPackage,xp2p))
