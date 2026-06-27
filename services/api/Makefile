.PHONY: lint lint-fix fmt

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

fmt:
	golangci-lint run --fix --disable-all --enable=gofmt,goimports,gofumpt ./...
