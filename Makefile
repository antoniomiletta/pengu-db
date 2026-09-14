.PHONY: dev

dev:
	go run ./cmd/pengu

test:
	go test ./... | grep -v "no test files"
