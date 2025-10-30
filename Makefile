run:
	go run ./go/cmd/xp2p

PING_HOST ?= 127.0.0.1
ping:
	go run ./go/cmd/xp2p ping $(PING_HOST)

build:
	go build -o bin/xp2p ./go/cmd/xp2p

fmt:
	go fmt ./...

lint:
	go vet ./...
