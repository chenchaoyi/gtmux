# Developer shortcuts. `make check` runs the same gate as CI (.github/workflows/ci.yml).
.PHONY: build app app-release install test cover fmt vet lint check clean

BIN      ?= gtmux
PKG       = ./cmd/gtmux
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# GTMUX_TUNNEL_REG bakes the hosted-tunnel registration gate into the binary
# (empty by default → hosted `gtmux tunnel` cleanly tells you to use --quick).
LDFLAGS   = -s -w -X github.com/chenchaoyi/gtmux/internal/app.Version=$(VERSION) -X github.com/chenchaoyi/gtmux/internal/app.TunnelRegSecret=$(GTMUX_TUNNEL_REG) -X github.com/chenchaoyi/gtmux/internal/app.RelayToken=$(GTMUX_RELAY_TOKEN)

build: ## Build the gtmux CLI (cgo-free) into ./$(BIN)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

app: ## Build the native menu-bar app (Gtmux.app) — Swift + the bundled CLI
	cd macapp && GTMUX_VERSION=$(VERSION) ./build.sh

app-release: ## Build a signed+notarized Gtmux.app and publish it to the release + cask (local path; see docs/release-signing.md)
	macapp/release.sh

install: ## Install gtmux into $GOBIN / $GOPATH/bin
	go install -ldflags "$(LDFLAGS)" $(PKG)

test: ## Run tests with the race detector
	go test -race ./...

cover: ## Run tests and print a coverage summary
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

fmt: ## Check formatting (fails if anything needs gofmt)
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

vet: ## go vet
	go vet ./...

lint: ## staticcheck (pinned via go run)
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

check: fmt vet lint test ## Run the full CI gate locally

clean: ## Remove build artifacts
	rm -f $(BIN) coverage.out
	rm -rf dist/ macapp/.build macapp/build
