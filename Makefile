.PHONY: all controller agent web proto test lint migrate-up migrate-down migrate-status bin

CONTROLLER_BIN := bin/controller
AGENT_BIN      := bin/agent.exe

all: web controller agent

controller: bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="-s -w" -o $(CONTROLLER_BIN) ./cmd/controller

agent: bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build -ldflags="-s -w" -o $(AGENT_BIN) ./cmd/agent

web:
	cd web && npm ci && npm run build

proto:
	protoc \
		--go_out=. --go_opt=module=github.com/badskater/distributed-encoder \
		--go-grpc_out=. --go-grpc_opt=module=github.com/badskater/distributed-encoder \
		proto/encoder/v1/agent.proto

test:
	go test ./... -race -cover -timeout 120s

lint:
	golangci-lint run ./...

migrate-up:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" down 1

migrate-status:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" version

bin:
	mkdir -p bin
