.PHONY: build test lint clean test-e2e test-e2e-ui

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

# E2E Tests (Playwright)
test-e2e:
	cd e2e && npm test

test-e2e-ui:
	cd e2e && npm run test:ui

test-e2e-install:
	cd e2e && npm install && npx playwright install chromium

clean:
	rm -f ../bin/romman-web
	rm -rf e2e/test-results e2e/playwright-report
