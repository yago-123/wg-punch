.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...