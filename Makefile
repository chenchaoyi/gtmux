# Developer shortcuts. `make check` runs the same gate as CI (.github/workflows/ci.yml).
.PHONY: build install test cover fmt vet lint check clean

BIN     ?= gtmux
PKG      = ./cmd/gtmux
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -s -w -X github.com/chenchaoyi/gtmux/internal/app.Version=$(VERSION)

build: ## Build the gtmux binary into ./$(BIN)
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

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
	rm -rf dist/
