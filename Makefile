run:
	go run ./go/cmd/xp2p

VERSION ?= $(strip $(shell go run ./go/cmd/xp2p --version))
GO_LDFLAGS := -s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$(VERSION)

VAGRANT_WIN10_DIR := infra/vagrant-win/windows10
VAGRANT_WIN10_SERVER_ID := win10-server
VAGRANT_WIN10_CLIENT_ID := win10-client
TARGETS := $(strip $(shell go run ./go/tools/targets list --scope all))
BUILD_BASE := build
.PHONY: run build build-% build-windows build-linux build-openwrt fmt lint test vagrant-win10 vagrant-win10-destroy \
	vagrant-win10-server vagrant-win10-client \
	vagrant-win10-destroy-server vagrant-win10-destroy-client

build: $(TARGETS:%=build-%)

build-%:
	powershell -NoProfile -Command "go run ./go/tools/targets build --target '$*' --base '$(BUILD_BASE)' --binary 'xp2p' --pkg './go/cmd/xp2p' --ldflags \"$(GO_LDFLAGS)\""

build-windows: build-windows-amd64

build-linux: build-linux-amd64

build-openwrt: build-linux-mipsle

fmt:
	go fmt ./...

lint:
	go vet ./...

test:
	go test ./...	

vagrant-win10:
	cd $(VAGRANT_WIN10_DIR) && vagrant up

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
