.PHONY: dev test

dev:
	go run ./cmd/app

test:
	go test ./... | grep -v "no test files"
