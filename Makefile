.PHONY : clearscr fresh clean all

run:
	. .env && go run ./cmd/tradebot/main.go

test:
	go test -v ./...

build-armv64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o tradebot ./cmd/tradebot/main.go

build: build-armv64
