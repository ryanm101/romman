.PHONY: build test lint clean

build:
	go build -o ../bin/romman-web .

test:
	go test ./...

test-short:
	go test ./...

lint:
	golangci-lint run ./...

test-cover:
	go test -coverprofile=../coverage-web.out ./...
	go tool cover -html=../coverage-web.out -o ../coverage-web.html

clean:
	rm -f ../bin/romman-web
