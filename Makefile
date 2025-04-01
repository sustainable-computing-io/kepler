.PHONY: all build clean test lint vet fmt image push

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

# Project parameters
BINARY_NAME=kepler
BINARY_DIR=bin
MAIN_GO_PATH=./cmd/kepler
VERSION=$(shell git describe --tags --always --dirty || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null)

# Docker parameters
IMG_BASE ?= quay.io/sustainable_computing_io

KEPLER_IMAGE ?= $(IMG_BASE)/kepler-reboot:$(VERSION)

# Test parameters
COVER_PROFILE=coverage.out
COVER_HTML=coverage.html

all: clean fmt lint vet build test

# Build the application
build:
	mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) $(MAIN_GO_PATH)

build-debug:
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -race $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) $(MAIN_GO_PATH)

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f $(COVER_PROFILE) $(COVER_HTML)

# Run tests with coverage
test:
	CGO_ENABLED=1 $(GOTEST) -v -race -coverprofile=$(COVER_PROFILE) ./...

# Generate coverage report
coverage: test
	$(GOCMD) tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)

# Run linting
lint:
	$(GOLINT) run ./...

# Run go vet
vet:
	$(GOVET) ./...

# Format code
fmt:
	$(GOFMT) ./...

# Tidy and verify dependencies
deps:
	$(GOMOD) tidy
	$(GOMOD) verify

# Build Docker image
image: test deps
	docker build -t \
		$(KEPLER_IMAGE) \
		--platform=linux/$(GOARCH) .

# Push Docker image
push:
	docker push $(KEPLER_IMAGE)

# Run the application
run:
	$(BINARY_DIR)/$(BINARY_NAME)

