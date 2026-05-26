.PHONY: build test bench vet lint run docker clean all

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X github.com/akzj/tau/core.Version=$(VERSION) -X github.com/akzj/tau/core.BuildTime=$(BUILD_TIME) -X github.com/akzj/tau/core.CommitSHA=$(COMMIT_SHA)

# Build the tau binary
build:
	go build -ldflags="$(LDFLAGS)" -o tau ./cmd/tau/

# Run all tests with race detector
test:
	go test -race -count=1 ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem -benchtime=100ms -run=^$$ ./...

# Run go vet
vet:
	go vet ./...

# Build and run
run: build
	./tau

# Docker build
docker:
	docker build -t tau .

# Full CI check (lint + test + vet + build)
all: vet test build
	@echo "All checks passed"

# Clean build artifacts
clean:
	rm -f tau
	go clean -cache -testcache
