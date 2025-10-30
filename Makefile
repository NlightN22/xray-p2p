run:
	go run ./go/cmd/xp2p

build:
	go build -o bin/xp2p ./go/cmd/xp2p

fmt:
	go fmt ./...

lint:
	go vet ./...
