.PHONY: build test lint tidy clean release

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

BINARY := netprobe
BUILD_DIR := ./build

default: build

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd

build-all:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd

test:
	go test -v -race -coverprofile=coverage.out ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

tidy:
	go mod tidy
	go mod verify

clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

install: build
	go install $(LDFLAGS) ./cmd

run: build
	./$(BINARY) -t google.com -p 443 -c 5

dev: build
	./$(BINARY) -t localhost -p 8080 -c 3

check: tidy lint test

release:
	goreleaser release --clean --snapshot