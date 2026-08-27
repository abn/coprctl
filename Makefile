.DEFAULT_GOAL := help

BIN := coprctl
PKG := ./cmd/$(BIN)

.PHONY: help build check fmt lint test clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o bin/$(BIN) $(PKG)

test: ## Run all tests
	go test ./...

lint: ## Run static analysis
	go vet ./...
	go fmt ./...

fmt: ## Format code
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

check: fmt lint test ## Full quality gate
	@echo "check: ok"

clean: ## Remove build artefacts
	rm -rf bin
