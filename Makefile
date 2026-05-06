.PHONY: all
all:
	@mise run all

.PHONY: lint
lint:
	@mise run lint

.PHONY: imports
imports:
	@mise run imports

.PHONY: fmt
fmt:
	@mise run fmt
