.PHONY: build test lint clean

build:
	go build -ldflags "-s -w" -o ../bin/romman-tui .

test:
	go test -v ./...

test-short:
	go test ./...

lint:
	golangci-lint run ./...

test-cover:
	go test -coverprofile=../coverage-tui.out ./...
	go tool cover -html=../coverage-tui.out -o ../coverage-tui.html

clean:
	rm -f ../bin/romman-tui
