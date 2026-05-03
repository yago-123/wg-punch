.PHONY: all
all: imports fmt lint

.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

.PHONY: imports
imports:
	@echo "Running imports..."
	@find . -name "*.go" | xargs goimports -w

.PHONY: fmt
fmt:
	@echo "Running fmt..."
	@go fmt ./...
