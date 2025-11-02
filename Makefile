run:
	go run ./go/cmd/xp2p

VAGRANT_WIN10_DIR := infra/vagrant-win/windows10
VAGRANT_WIN10_SERVER_ID := win10-server
VAGRANT_WIN10_CLIENT_ID := win10-client
WINDOWS_BUILD_DIR := build/windows-amd64
LINUX_BUILD_DIR := build/linux-amd64
OPENWRT_BUILD_DIR := build/linux-mipsle-softfloat

.PHONY: ping _ping_args wrm-test run build build-windows build-linux build-openwrt fmt lint vagrant-win10 vagrant-win10-destroy \
	vagrant-win10-server vagrant-win10-client \
	vagrant-win10-destroy-server vagrant-win10-destroy-client

ping: _ping_args
	@set -- $(ARGS); \
	if [ -z "$$1" ]; then \
		echo "Usage: make ping <host> [proto] [port]"; exit 1; \
	fi; \
	HOST="$$1"; shift || true; \
	PROTO="$$1"; if [ -n "$$PROTO" ]; then shift || true; fi; \
	PORT="$$1"; \
	CMD="go run ./go/cmd/xp2p ping"; \
	if [ -n "$$PROTO" ]; then CMD="$$CMD --proto $$PROTO"; fi; \
	if [ -n "$$PORT" ]; then CMD="$$CMD --port $$PORT"; fi; \
	CMD="$$CMD $$HOST"; \
	echo $$CMD; \
	eval $$CMD

_ping_args:
	$(eval ARGS := $(filter-out ping _ping_args,$(MAKECMDGOALS)))

VM ?= win10-server

wrm-test: build-windows
	@if [ -z "$(CMD)" ]; then \
		echo "Usage: make wrm-test CMD=\"ping 10.0.10.10 --port 62022\" [VM=win10-server]"; \
		exit 1; \
	fi; \
	echo "==> VM: $(VM)"; \
	echo "==> CMD: $(CMD)"; \
	python $(subst \,/,$(abspath scripts/xp2p_winrm.py)) --vm $(VM) -- $(CMD)

build: build-windows build-linux build-openwrt

build-windows:
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(WINDOWS_BUILD_DIR)' | Out-Null; $$env:GOOS = 'windows'; $$env:GOARCH = 'amd64'; go build -o '$(WINDOWS_BUILD_DIR)/xp2p.exe' ./go/cmd/xp2p; Remove-Item Env:GOOS -ErrorAction SilentlyContinue; Remove-Item Env:GOARCH -ErrorAction SilentlyContinue"

build-linux:
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(LINUX_BUILD_DIR)' | Out-Null; $$env:GOOS = 'linux'; $$env:GOARCH = 'amd64'; go build -o '$(LINUX_BUILD_DIR)/xp2p' ./go/cmd/xp2p; Remove-Item Env:GOOS -ErrorAction SilentlyContinue; Remove-Item Env:GOARCH -ErrorAction SilentlyContinue"

build-openwrt:
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(OPENWRT_BUILD_DIR)' | Out-Null; $$env:GOOS = 'linux'; $$env:GOARCH = 'mipsle'; $$env:GOMIPS = 'softfloat'; go build -o '$(OPENWRT_BUILD_DIR)/xp2p' ./go/cmd/xp2p; Remove-Item Env:GOOS -ErrorAction SilentlyContinue; Remove-Item Env:GOARCH -ErrorAction SilentlyContinue; Remove-Item Env:GOMIPS -ErrorAction SilentlyContinue"

fmt:
	go fmt ./...

lint:
	go vet ./...

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
