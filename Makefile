.DEFAULT_GOAL := help

BIN := coprctl
PKG := ./cmd/$(BIN)
GENERATED := docs/reference/commands.md skills/coprctl/SKILL.md skills/coprctl-debug/SKILL.md

.PHONY: help build check fmt lint test clean gen drift

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

gen: ## Regenerate generated artefacts from the command registry
	go run ./cmd/coprctl schema --format markdown > docs/reference/commands.md
	go run ./cmd/coprctl skill print coprctl > skills/coprctl/SKILL.md
	go run ./cmd/coprctl skill print coprctl-debug > skills/coprctl-debug/SKILL.md

drift: gen ## Fail if generated artefacts differ from the committed versions
	@for f in $(GENERATED); do \
		git ls-files --error-unmatch "$$f" >/dev/null 2>&1 || { echo "drift: $$f is not tracked (run git add)"; exit 1; }; \
		git diff --exit-code --quiet "$$f" || { echo "drift: $$f is out of date (run make gen)"; exit 1; }; \
	done
	@echo "drift: ok"

check: fmt lint test drift ## Full quality gate
	@echo "check: ok"

clean: ## Remove build artefacts
	rm -rf bin
