.PHONY: all controller agent agent-linux agent-windows web proto test lint \
        migrate-up migrate-down migrate-status bin installer \
        deb deb-agent rpm rpm-agent \
        desktop-windows desktop-linux deb-desktop

VERSION          ?= dev
LDFLAGS          := -s -w -X main.Version=$(VERSION)

CONTROLLER_BIN   := bin/controller
AGENT_BIN        := bin/agent.exe
AGENT_LINUX_BIN  := bin/agent
DESKTOP_WIN_BIN  := bin/encodeswarmr-desktop.exe
DESKTOP_LINUX_BIN := bin/encodeswarmr-desktop

all: web controller agent

controller: bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS)" -o $(CONTROLLER_BIN) ./cmd/controller

# Windows agent binary (default — used by install-agent.ps1 and Inno Setup installer)
agent: agent-windows

agent-windows: bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS)" -o $(AGENT_BIN) ./cmd/agent

# Linux agent binary — required for deb-agent and rpm-agent targets
agent-linux: bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS)" -o $(AGENT_LINUX_BIN) ./cmd/agent

web:
	cd web && npm ci && npm run build

proto:
	protoc \
		--go_out=. --go_opt=module=github.com/badskater/encodeswarmr \
		--go-grpc_out=. --go-grpc_opt=module=github.com/badskater/encodeswarmr \
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

# Windows only — requires Inno Setup 6: choco install innosetup
# Override ISCC if installed to a non-default path.
ISCC ?= C:\Program Files (x86)\Inno Setup 6\ISCC.exe

installer:
	"$(ISCC)" /DAgentVersion=$(VERSION) /O"dist" installer\agent-setup.iss

# ── Linux packages — requires nFPM: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest ──

# .deb packages (Debian / Ubuntu)
deb: controller
	VERSION=$(VERSION) nfpm package --config nfpm.yaml --packager deb --target dist/

deb-agent: agent-linux
	VERSION=$(VERSION) nfpm package --config nfpm-agent.yaml --packager deb --target dist/

# .rpm packages (RHEL / Rocky Linux / AlmaLinux / Fedora)
rpm: controller
	VERSION=$(VERSION) nfpm package --config nfpm-rpm.yaml --packager rpm --target dist/

rpm-agent: agent-linux
	VERSION=$(VERSION) nfpm package --config nfpm-agent-rpm.yaml --packager rpm --target dist/

# ── Desktop GUI client ──────────────────────────────────────────────────────────

desktop-windows: bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS) -H=windowsgui" -o $(DESKTOP_WIN_BIN) ./cmd/desktop

desktop-linux: bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS)" -o $(DESKTOP_LINUX_BIN) ./cmd/desktop

deb-desktop: desktop-linux
	VERSION=$(VERSION) nfpm package --config nfpm-desktop.yaml --packager deb --target dist/
