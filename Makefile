run:
	go run ./go/cmd/xp2p

VAGRANT_WIN10_DIR := infra/vagrant-win/windows10
VAGRANT_WIN10_SERVER_ID := win10-server
VAGRANT_WIN10_CLIENT_ID := win10-client

VAGRANT_WIN10_ROLE := all
ifneq ($(filter --server,$(MAKECMDGOALS)),)
VAGRANT_WIN10_ROLE := server
endif
ifneq ($(filter --client,$(MAKECMDGOALS)),)
VAGRANT_WIN10_ROLE := client
endif
ifneq ($(filter --all,$(MAKECMDGOALS)),)
VAGRANT_WIN10_ROLE := all
endif

.PHONY: ping _ping_args run build fmt lint vagrant-win10 vagrant-win10-destroy

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

build:
	go build -o bin/xp2p ./go/cmd/xp2p

fmt:
	go fmt ./...

lint:
	go vet ./...

vagrant-win10:
	@if [ "$(VAGRANT_WIN10_ROLE)" = "server" ]; then \
		cd $(VAGRANT_WIN10_DIR) && vagrant up $(VAGRANT_WIN10_SERVER_ID) --provision; \
	elif [ "$(VAGRANT_WIN10_ROLE)" = "client" ]; then \
		cd $(VAGRANT_WIN10_DIR) && vagrant up $(VAGRANT_WIN10_CLIENT_ID) --provision; \
	else \
		cd $(VAGRANT_WIN10_DIR) && vagrant up --provision; \
	fi

vagrant-win10-destroy:
	@if [ "$(VAGRANT_WIN10_ROLE)" = "server" ]; then \
		cd $(VAGRANT_WIN10_DIR) && vagrant destroy -f $(VAGRANT_WIN10_SERVER_ID); \
	elif [ "$(VAGRANT_WIN10_ROLE)" = "client" ]; then \
		cd $(VAGRANT_WIN10_DIR) && vagrant destroy -f $(VAGRANT_WIN10_CLIENT_ID); \
	else \
		cd $(VAGRANT_WIN10_DIR) && vagrant destroy -f; \
	fi

# swallow extra positional arguments so make does not treat them as targets
%:
	@:
