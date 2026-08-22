.PHONY: dev

dev:
	go run cmd/pengu-db/main.go

test:
	go test ./... | grep -v "no test files"
