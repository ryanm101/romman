.PHONY: test lint

test:
	go test -v ./...

test-short:
	go test ./...

lint:
	golangci-lint run ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
