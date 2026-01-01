# Milestone 13: Code Quality & Test Coverage

Focus on improving code quality, increasing test coverage, and refactoring large files.

## Goals
- Increase overall test coverage to 60%+
- Add missing tests for critical packages
- Refactor large files for maintainability

## Tasks

### Test Coverage
- [ ] Add `config/config_test.go` - test loading, defaults, env overrides, path resolution
- [ ] Add `tracing/tracing_test.go` - test span creation, config handling
- [ ] Add `metrics/metrics_test.go` - test gauge updates, counter increments
- [ ] Add `logging/logger_test.go` - test format/level configuration
- [ ] Add `dat/parser_test.go` - test XML edge cases, malformed input
- [ ] Improve `library/` coverage to 60%+ - scanner parallel mode, matching edge cases

### Refactoring
- [ ] Split `romman-cli/main.go` into command-specific files
- [ ] Split `library/scanner.go` - extract file hashing, zip handling
- [ ] Extract SQL queries into constants or query builder
- [ ] Standardize error handling patterns across CLI

### Code Quality
- [ ] Add godoc comments to all exported functions
- [ ] Replace global `cfg` variable with explicit parameter passing
- [ ] Add `go vet` and `staticcheck` to CI pipeline
- [ ] Add golangci-lint configuration

## Success Criteria
- All packages have test files
- `go test -cover ./...` shows 60%+ overall
- `golangci-lint run` passes with minimal issues
