.PHONY: build test lint clean

build:
	go build -ldflags "-s -w" -o ../bin/romman-tui .

test:
	go test -v ./...

test-short:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f ../bin/romman-tui
