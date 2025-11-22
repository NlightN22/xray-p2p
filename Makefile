run:
	go run ./go/cmd/xp2p

VERSION ?= $(strip $(shell go run ./go/cmd/xp2p --version))
GO_LDFLAGS := -s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$(VERSION)

VAGRANT_WIN10_DIR := infra/vagrant/windows10
VAGRANT_WIN10_SERVER_ID := win10-server
VAGRANT_WIN10_CLIENT_ID := win10-client

VAGRANT_DEB12_DIR := infra/vagrant/debian12

TARGETS := $(strip $(shell go run ./go/tools/targets list --scope all))
BUILD_BASE := build
.PHONY: run build build-% fmt lint test vagrant-win10 vagrant-win10-destroy \
	vagrant-win10-server vagrant-win10-client \
	vagrant-win10-destroy-server vagrant-win10-destroy-client

build: $(TARGETS:%=build-%)

build-%:
	powershell -NoProfile -Command "go run ./go/tools/targets build --target '$*' --base '$(BUILD_BASE)' --binary 'xp2p' --pkg './go/cmd/xp2p' --ldflags \"$(GO_LDFLAGS)\"; go run ./go/tools/targets deps --target '$*' --base '$(BUILD_BASE)'"

fmt:
	go fmt ./...

lint:
	go vet ./...

test:
	powershell -NoProfile -Command "go clean -testcache ; go test ./... -cover"

test-wsl:
	wsl bash -lc "cd /mnt/d/Programming/Go/xray-p2p && go clean -testcache && go test ./... -cover"

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

vagrant-deb12:
	cd $(VAGRANT_DEB12_DIR) && vagrant up

# swallow extra positional arguments so make does not treat them as targets
%:
	@:
