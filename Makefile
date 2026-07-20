run:
	go run ./go/cmd/xp2p

VERSION ?= $(strip $(shell go run ./go/cmd/xp2p --version))
GO_LDFLAGS := -s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$(VERSION)

VAGRANT_WINMSI_DIR := infra/vagrant/win-msi

VAGRANT_WIN10_DIR := infra/vagrant/windows10
VAGRANT_WIN11_DIR := infra/vagrant/windows11
VAGRANT_WIN22_DIR := infra/vagrant/server2022
VAGRANT_WIN16_DIR := infra/vagrant/server2016
VAGRANT_WIN10_SERVER_ID := win10-a
VAGRANT_WIN10_CLIENT_ID := win10-b

VAGRANT_DEB12_DIR := infra/vagrant/debian12/deb-test

VAGRANT_IPK_BUILD_DIR := infra/vagrant/debian12/ipk-build
VAGRANT_OWRT_DIR := infra/vagrant/openwrt

TARGETS := $(strip $(shell go run ./go/tools/targets list --scope all))
BUILD_BASE := build
WSL_CD = drive=$$(printf '%s' '$(CURDIR)' | cut -c1 | tr '[:upper:]' '[:lower:]'); path=$$(printf '%s' '$(CURDIR)' | cut -c3-); repo_dir=/mnt/$$drive$$path; cd \"$$repo_dir\"
.PHONY: run build build-% fmt lint test schema schema-check schema-test schema-compat vagrant-win10 vagrant-win10-destroy up-deb-test-c halt-deb-test-c provision-deb-test-c \
	vagrant-win10-server vagrant-win10-client \
	vagrant-win10-destroy-server vagrant-win10-destroy-client build-ipk build-ipk-infra build-deb build-msi \
	ui-native-build ui-native-test ui-native-cover ui-native-test-cover-wsl test-wsl command-map

build: $(TARGETS:%=build-%)

build-%:
	powershell -NoProfile -Command "go run ./go/tools/targets build --target '$*' --base '$(BUILD_BASE)' --binary 'xp2p' --pkg './go/cmd/xp2p' --ldflags \"$(GO_LDFLAGS)\"; go run ./go/tools/targets deps --target '$*' --base '$(BUILD_BASE)'"

fmt:
	go fmt ./...

lint:
	go vet ./...
	$(MAKE) schema-check
	$(MAKE) schema-test

schema:
	go run ./go/tools/configschema --output schemas

schema-check:
	go run ./go/tools/configschema --output schemas --check

schema-test:
	npm run lint:config-schema

schema-compat:
	npm run lint:config-schema -- tests/schema/compat/v0.2.6/xp2p-client.toml tests/schema/compat/v0.2.6/xp2p-server.toml
	go test ./go/internal/config ./go/internal/client ./go/internal/server ./go/internal/xrayconfig -run SchemaCompatibility

test:
	powershell -NoProfile -Command "go clean -testcache ; go test ./... -cover"

ui-native-test-cover-wsl:
	wsl bash -lc "set -euo pipefail; $(WSL_CD); rm -rf build/cpp/ui-xp2p-cover-wsl; cmake -S cpp/ui-xp2p -B build/cpp/ui-xp2p-cover-wsl -G Ninja -DXP2P_UI_BUILD_TESTS=ON -DXP2P_UI_ENABLE_COVERAGE=ON; cmake --build build/cpp/ui-xp2p-cover-wsl; ctest --test-dir build/cpp/ui-xp2p-cover-wsl --output-on-failure; mkdir -p build/cpp/ui-xp2p-cover-wsl/coverage; gcovr -r . --object-directory build/cpp/ui-xp2p-cover-wsl --filter 'cpp/ui-xp2p/src' --exclude 'cpp/ui-xp2p/src/(tray_app|service_manager|path_utils|logging)\\.cpp' --html-details --output build/cpp/ui-xp2p-cover-wsl/coverage/index.html; gcovr -r . --object-directory build/cpp/ui-xp2p-cover-wsl --filter 'cpp/ui-xp2p/src' --exclude 'cpp/ui-xp2p/src/(tray_app|service_manager|path_utils|logging)\\.cpp' --xml-pretty --output build/cpp/ui-xp2p-cover-wsl/coverage/coverage.xml; gcovr -r . --object-directory build/cpp/ui-xp2p-cover-wsl --filter 'cpp/ui-xp2p/src' --exclude 'cpp/ui-xp2p/src/(tray_app|service_manager|path_utils|logging)\\.cpp'"

ui-native-build:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build/build_ui_native.ps1 -Task build -Config Release

ui-native-test:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build/build_ui_native.ps1 -Task test -Config Debug

ui-native-cover:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build/build_ui_native.ps1 -Task cover -Config Debug

test-wsl:
	wsl bash -lc "$(WSL_CD) && go clean -testcache && go test ./... -cover"

command-map:
	wsl bash -lc "set -euo pipefail; $(WSL_CD); rm -rf commands_map; go run ./go/cmd/xp2p docs command-map --dir commands_map"

up-win10:
	cd $(VAGRANT_WIN10_DIR) && vagrant up
	
halt-win10:
	cd $(VAGRANT_WIN10_DIR) && vagrant halt

up-win11:
	cd $(VAGRANT_WIN11_DIR) && vagrant up
	
halt-win11:
	cd $(VAGRANT_WIN11_DIR) && vagrant halt

up-win22:
	cd $(VAGRANT_WIN22_DIR) && vagrant up

halt-win22:
	cd $(VAGRANT_WIN22_DIR) && vagrant halt

up-win16:
	cd $(VAGRANT_WIN16_DIR) && vagrant up

halt-win16:
	cd $(VAGRANT_WIN16_DIR) && vagrant halt

up-deb12:
	cd $(VAGRANT_DEB12_DIR) && vagrant up
halt-deb12:
	cd $(VAGRANT_DEB12_DIR) && vagrant halt

up-owrt:
	cd $(VAGRANT_IPK_BUILD_DIR) && vagrant up
	cd $(VAGRANT_OWRT_DIR) && vagrant up

provision-build-owrt:
	cd $(VAGRANT_IPK_BUILD_DIR) && vagrant ssh -c "/srv/xray-p2p/scripts/build/build_openwrt_ipk.sh --target linux-amd64 --output-dir /srv/xray-p2p/build/ipk --force-build"
	cd $(VAGRANT_OWRT_DIR) && vagrant up --provision

halt-owrt:
	cd $(VAGRANT_IPK_BUILD_DIR) && vagrant halt
	cd $(VAGRANT_OWRT_DIR) && vagrant halt

build-ipk:
	$(MAKE) test
	$(MAKE) test-wsl
	$(MAKE) lint
	cd $(VAGRANT_IPK_BUILD_DIR) && vagrant up
	cd $(VAGRANT_IPK_BUILD_DIR) && vagrant ssh -c "/srv/xray-p2p/scripts/build/build_openwrt_ipk.sh --all --force-build"

build-ipk-infra:
	cd $(VAGRANT_IPK_BUILD_DIR) && vagrant ssh -c "/srv/xray-p2p/scripts/build/build_openwrt_ipk.sh --target linux-amd64 --output-dir /srv/xray-p2p/build/ipk --force-build"
	cd $(VAGRANT_OWRT_DIR) && vagrant up --provision

build-deb:
	$(MAKE) test
	$(MAKE) test-wsl
	$(MAKE) lint
	cd $(VAGRANT_DEB12_DIR) && vagrant up deb-test-a
	cd $(VAGRANT_DEB12_DIR) && vagrant ssh deb-test-a -c "sudo -n /bin/bash /srv/xray-p2p/scripts/build/build_deb_xp2p.sh --all"

up-deb-test-c:
	cd $(VAGRANT_DEB12_DIR) && vagrant up deb-test-c

halt-deb-test-c:
	cd $(VAGRANT_DEB12_DIR) && vagrant halt deb-test-c

provision-deb-test-c:
	cd $(VAGRANT_DEB12_DIR) && vagrant provision deb-test-c

# swallow extra positional arguments so make does not treat them as targets
%:
	@:
