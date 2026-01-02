.PHONY: build test lint clean

build:
	go build -ldflags "-s -w" -o ../bin/romman .

test:
	go test -v ./...

test-short:
	go test ./...

lint:
	golangci-lint run ./...

test-cover:
	go test -coverprofile=../coverage-cli.out ./...
	go tool cover -html=../coverage-cli.out -o ../coverage-cli.html

clean:
	rm -f ../bin/romman
