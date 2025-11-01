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

ifeq ($(VAGRANT_WIN10_ROLE),server)
VAGRANT_WIN10_UP_CMD := vagrant up $(VAGRANT_WIN10_SERVER_ID) --provision
VAGRANT_WIN10_DESTROY_CMD := vagrant destroy -f $(VAGRANT_WIN10_SERVER_ID)
else ifeq ($(VAGRANT_WIN10_ROLE),client)
VAGRANT_WIN10_UP_CMD := vagrant up $(VAGRANT_WIN10_CLIENT_ID) --provision
VAGRANT_WIN10_DESTROY_CMD := vagrant destroy -f $(VAGRANT_WIN10_CLIENT_ID)
else
VAGRANT_WIN10_UP_CMD := vagrant up --provision
VAGRANT_WIN10_DESTROY_CMD := vagrant destroy -f
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
	cd $(VAGRANT_WIN10_DIR) && $(VAGRANT_WIN10_UP_CMD)

vagrant-win10-destroy:
	cd $(VAGRANT_WIN10_DIR) && $(VAGRANT_WIN10_DESTROY_CMD)

# swallow extra positional arguments so make does not treat them as targets
%:
	@:
