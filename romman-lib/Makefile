.PHONY: test lint

test:
	go test -v ./...

test-short:
	go test ./...

lint:
	golangci-lint run ./...

test-cover:
	go test -coverprofile=../coverage-lib.out ./...
	go tool cover -html=../coverage-lib.out -o ../coverage-lib.html
