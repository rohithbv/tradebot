.PHONY: run test build build-armv64 docker-build docker-run docker-up docker-down deploy

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
	docker run --env-file .env -e TRADEBOT_DB_PATH=/data/tradebot.db -p 8080:8080 -v tradebot-data:/data tradebot

docker-up:
	docker compose up -d

docker-down:
	docker compose down

deploy:
ifndef REMOTE_HOST
	$(error REMOTE_HOST is required. Usage: make deploy REMOTE_HOST=user@host)
endif
	./deploy.sh $(REMOTE_HOST)
