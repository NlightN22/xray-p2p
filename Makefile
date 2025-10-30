run:
	go run ./go/cmd/xp2p

.PHONY: ping _ping_args run build fmt lint tag-release

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

tag-release:
	@if [ -z "$(TAG)" ]; then \
		echo "Usage: make tag-release TAG=vX.Y.Z [TAG_MESSAGE='message'] [FORCE=1]"; \
		exit 1; \
	fi; \
	force_flag=""; \
	if git rev-parse "$(TAG)" >/dev/null 2>&1; then \
		if [ "$(FORCE)" = "1" ]; then \
			force_flag="--force"; \
			msg=$${TAG_MESSAGE:-"Release $(TAG)"}; \
			git tag -a -f "$(TAG)" -m "$$msg"; \
			echo "Updated tag $(TAG) with force"; \
		else \
			echo "Tag $(TAG) already exists locally. Use FORCE=1 to overwrite."; \
		fi; \
	else \
		msg=$${TAG_MESSAGE:-"Release $(TAG)"}; \
		git tag -a "$(TAG)" -m "$$msg"; \
		echo "Created tag $(TAG)"; \
	fi; \
	if [ "$(FORCE)" = "1" ]; then \
		git push --force origin "$(TAG)"; \
	else \
		git push origin "$(TAG)"; \
	fi

# swallow extra positional arguments so make does not treat them as targets
%:
	@:
