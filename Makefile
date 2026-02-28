.PHONY: run test build build-armv64 docker-build docker-run docker-up docker-down

run:
	. .env && go run ./cmd/tradebot/main.go

test:
	go test -v ./...

build-armv64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o tradebot ./cmd/tradebot/main.go

build: build-armv64

docker-build:
	docker build -t tradebot .

docker-run:
	docker run --env-file .env -p 8080:8080 -v tradebot-data:/data tradebot

docker-up:
	docker compose up -d

docker-down:
	docker compose down
