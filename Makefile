.PHONY: all build test lint clean test-e2e

all: build-all

# Build CLI (default)
build:
	$(MAKE) -C romman-cli build

# Build all binaries
build-all:
	$(MAKE) -C romman-cli build
	$(MAKE) -C romman-tui build
	$(MAKE) -C romman-web build

# Tests
test:
	$(MAKE) -C romman-lib test
	$(MAKE) -C romman-cli test
	$(MAKE) -C romman-tui test
	$(MAKE) -C romman-web test

test-short:
	$(MAKE) -C romman-lib test-short
	$(MAKE) -C romman-cli test-short
	$(MAKE) -C romman-tui test-short
	$(MAKE) -C romman-web test-short

# E2E Tests (Playwright - requires running server)
test-e2e:
	$(MAKE) -C romman-web test-e2e

test-e2e-install:
	$(MAKE) -C romman-web test-e2e-install

test-cover:
	$(MAKE) -C romman-lib test-cover
	$(MAKE) -C romman-cli test-cover
	$(MAKE) -C romman-tui test-cover
	$(MAKE) -C romman-web test-cover

# Lint
lint:
	$(MAKE) -C romman-lib lint
	$(MAKE) -C romman-cli lint
	$(MAKE) -C romman-tui lint
	$(MAKE) -C romman-web lint

clean:
	$(MAKE) -C romman-cli clean
	$(MAKE) -C romman-tui clean
	$(MAKE) -C romman-web clean
	rm -rf bin/

# Convenience
dev: test-short build-all
