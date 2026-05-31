# Trellis — developer task runner.
# Run `make` (or `make help`) to list available targets.

# Use bash so ADR numbering can rely on base-10 arithmetic (10#...).
SHELL := /bin/bash
GO ?= go
ADR_DIR := docs/architecture/decisions

.DEFAULT_GOAL := help
.PHONY: help build vet test cover run bench clean adr adr-list

help: ## List available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Compile all packages
	$(GO) build ./...

vet: ## Run go vet
	$(GO) vet ./...

test: ## Run all tests
	$(GO) test ./...

cover: ## Run tests with a coverage summary
	$(GO) test -cover ./...

run: ## Run the demo binary
	$(GO) run ./cmd/trellis

bench: ## Run benchmarks with allocation stats
	$(GO) test -run=^$$ -bench=. -benchmem ./pkg/...

clean: ## Remove build artifacts and the demo database
	rm -f trellis trellis.db

adr-list: ## List existing ADRs
	@ls -1 $(ADR_DIR)/[0-9]*.md 2>/dev/null || echo "No ADRs yet."

adr: ## Create a new ADR from the template: make adr title="Short decision title"
	@if [ -z "$(title)" ]; then \
		echo 'usage: make adr title="Short decision title"'; exit 2; \
	fi
	@mkdir -p $(ADR_DIR)
	@num=$$(ls $(ADR_DIR)/[0-9]*.md 2>/dev/null | sed -E 's:.*/([0-9]+)-.*:\1:' | sort -n | tail -1); \
	next=$$(printf '%04d' $$((10#$${num:-0} + 1))); \
	slug=$$(printf '%s' "$(title)" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$$//'); \
	file=$(ADR_DIR)/$$next-$$slug.md; \
	awk -v n="$$next" -v t="$(title)" -v d="$$(date +%F)" -v s="Proposed" \
		'{ gsub(/{{NUMBER}}/, n); gsub(/{{TITLE}}/, t); gsub(/{{DATE}}/, d); gsub(/{{STATUS}}/, s); print }' \
		$(ADR_DIR)/template.md > "$$file"; \
	echo "created $$file"
