.PHONY: lint lint-fix fmt

lint:
	cd services/api && golangci-lint run ./...

lint-fix:
	cd services/api && golangci-lint run --fix ./...

fmt:
	cd services/api && golangci-lint run --fix --disable-all --enable=gofmt,goimports,gofumpt ./...
