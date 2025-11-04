run:
	go run ./go/cmd/xp2p

VERSION ?= $(strip $(shell go run ./go/cmd/xp2p --version))
GO_LDFLAGS := -s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$(VERSION)

VAGRANT_WIN10_DIR := infra/vagrant-win/windows10
VAGRANT_WIN10_SERVER_ID := win10-server
VAGRANT_WIN10_CLIENT_ID := win10-client
WINDOWS_BUILD_DIR := build/windows-amd64
LINUX_BUILD_DIR := build/linux-amd64
OPENWRT_BUILD_DIR := build/linux-mipsle-softfloat
.PHONY: run build build-windows build-linux build-openwrt fmt lint test vagrant-win10 vagrant-win10-destroy \
	vagrant-win10-server vagrant-win10-client \
	vagrant-win10-destroy-server vagrant-win10-destroy-client

build: build-windows build-linux build-openwrt

build-windows:
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(WINDOWS_BUILD_DIR)' | Out-Null; $$env:GOOS = 'windows'; $$env:GOARCH = 'amd64'; go build -trimpath -ldflags '$(GO_LDFLAGS)' -o '$(WINDOWS_BUILD_DIR)/xp2p.exe' ./go/cmd/xp2p; Remove-Item Env:GOOS -ErrorAction SilentlyContinue; Remove-Item Env:GOARCH -ErrorAction SilentlyContinue"

build-linux:
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(LINUX_BUILD_DIR)' | Out-Null; $$env:GOOS = 'linux'; $$env:GOARCH = 'amd64'; go build -trimpath -ldflags '$(GO_LDFLAGS)' -o '$(LINUX_BUILD_DIR)/xp2p' ./go/cmd/xp2p; Remove-Item Env:GOOS -ErrorAction SilentlyContinue; Remove-Item Env:GOARCH -ErrorAction SilentlyContinue"

build-openwrt:
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(OPENWRT_BUILD_DIR)' | Out-Null; $$env:GOOS = 'linux'; $$env:GOARCH = 'mipsle'; $$env:GOMIPS = 'softfloat'; go build -trimpath -ldflags '$(GO_LDFLAGS)' -o '$(OPENWRT_BUILD_DIR)/xp2p' ./go/cmd/xp2p; Remove-Item Env:GOOS -ErrorAction SilentlyContinue; Remove-Item Env:GOARCH -ErrorAction SilentlyContinue; Remove-Item Env:GOMIPS -ErrorAction SilentlyContinue"

fmt:
	go fmt ./...

lint:
	go vet ./...

test:
	go test ./...	

vagrant-win10:
	cd $(VAGRANT_WIN10_DIR) && vagrant up --provision

vagrant-win10-destroy:
	cd $(VAGRANT_WIN10_DIR) && vagrant destroy -f

vagrant-win10-server:
	cd $(VAGRANT_WIN10_DIR) && vagrant up $(VAGRANT_WIN10_SERVER_ID) --provision

vagrant-win10-client:
	cd $(VAGRANT_WIN10_DIR) && vagrant up $(VAGRANT_WIN10_CLIENT_ID) --provision

vagrant-win10-destroy-server:
	cd $(VAGRANT_WIN10_DIR) && vagrant destroy -f $(VAGRANT_WIN10_SERVER_ID)

vagrant-win10-destroy-client:
	cd $(VAGRANT_WIN10_DIR) && vagrant destroy -f $(VAGRANT_WIN10_CLIENT_ID)

# swallow extra positional arguments so make does not treat them as targets
%:
	@:
