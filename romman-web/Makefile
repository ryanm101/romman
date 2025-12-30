.PHONY: build test lint clean

build:
	go build -o ../bin/romman-web .

test:
	go test ./...

test-short:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f ../bin/romman-web
