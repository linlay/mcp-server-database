APP_NAME := mcp-server-database
BIN_NAME := mcp-server-database

.PHONY: build run test test-integration docker-build docker-up docker-down clean

build:
	go build -o ./bin/$(BIN_NAME) ./cmd/mcp-server

run:
	set -a; [ ! -f .env ] || . ./.env; set +a; SERVER_PORT="$${HOST_PORT:-$${SERVER_PORT:-8080}}" go run ./cmd/mcp-server

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down -v

clean:
	rm -f ./bin/$(BIN_NAME)
