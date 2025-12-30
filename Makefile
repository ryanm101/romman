.PHONY: all build test lint clean

all: build

# Top-level targets that call workspace-specific makefiles
build:
	$(MAKE) -C romman-cli build

test:
	$(MAKE) -C romman-lib test
	$(MAKE) -C romman-cli test

test-short:
	$(MAKE) -C romman-lib test-short
	$(MAKE) -C romman-cli test-short

lint:
	$(MAKE) -C romman-lib lint
	$(MAKE) -C romman-cli lint

clean:
	$(MAKE) -C romman-cli clean
	rm -rf bin/

# Convenience targets
dev: test-short build

# Build all binaries
build-all: build

# Run the CLI
run:
	./bin/romman $(ARGS)
