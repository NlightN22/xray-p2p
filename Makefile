run:
	go run ./go/cmd/xp2p

PING_HOST ?= 127.0.0.1
PING_PROTO ?= tcp
PING_PORT ?=
ping:
	go run ./go/cmd/xp2p ping $(if $(PING_PROTO),--proto $(PING_PROTO)) $(if $(PING_PORT),--port $(PING_PORT)) $(PING_HOST)

build:
	go build -o bin/xp2p ./go/cmd/xp2p

fmt:
	go fmt ./...

lint:
	go vet ./...
