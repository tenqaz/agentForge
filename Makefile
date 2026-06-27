.PHONY: lint lint-fix fmt

GOLANGCI_LINT := $(shell which /opt/homebrew/bin/golangci-lint 2>/dev/null || which golangci-lint)

lint:
	cd services/api && $(GOLANGCI_LINT) run ./...

lint-fix:
	cd services/api && $(GOLANGCI_LINT) run --fix ./...

fmt:
	cd services/api && $(GOLANGCI_LINT) fmt ./...
