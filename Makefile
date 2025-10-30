run:
	go run ./go/cmd/xp2p

.PHONY: ping _ping_args run build fmt lint

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

# swallow extra positional arguments so make does not treat them as targets
%:
	@:
