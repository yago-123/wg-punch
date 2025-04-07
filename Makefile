.PHONY: all
all: lint imports fmt

.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

.PHONY: imports
imports:
	@find . -name "*.go" | xargs goimports -w

.PHONY: fmt
fmt:
	@go fmt ./...

