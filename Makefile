.PHONY: all build build-debug clean test coverage lint vet fmt deps image push run

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

PRODUCTION ?= 0
ifeq ($(PRODUCTION), 1)
	# add -release suffix to binary name
	BINARY_NAME:=$(BINARY_NAME)-release
endif

BINARY_DIR=bin
MAIN_GO_PATH=./cmd/kepler
VERSION=$(shell git describe --tags --always --dirty | sed 's/-reboot//' || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null)

LD_VERSION_FLAGS=\
	-X github.com/sustainable-computing-io/kepler/internal/version.version=$(VERSION) \
	-X github.com/sustainable-computing-io/kepler/internal/version.buildTime=$(BUILD_TIME) \
	-X github.com/sustainable-computing-io/kepler/internal/version.gitBranch=$(GIT_BRANCH) \
	-X github.com/sustainable-computing-io/kepler/internal/version.gitCommit=$(GIT_COMMIT)

ifeq ($(PRODUCTION), 1)
	# strip debug symbols from production builds (to reduce binary size)
	LD_STRIP_DEBUG_SYMBOLS=-s -w
endif

LDFLAGS=-ldflags "$(LD_STRIP_DEBUG_SYMBOLS) $(LD_VERSION_FLAGS)"

BUILD_DEBUG_ARGS ?=

# Docker parameters
IMG_BASE ?= quay.io/sustainable_computing_io
KEPLER_IMAGE ?= $(IMG_BASE)/kepler-reboot:$(VERSION)
ADDITIONAL_TAGS ?=

# Test parameters
TEST_PKGS:= $(shell go list ./... | grep -v cmd)
COVER_PROFILE=coverage.out
COVER_HTML=coverage.html


all: clean fmt lint vet build test

# Build the application
build:
	mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(BUILD_ARGS) \
		$(LDFLAGS) \
		-o $(BINARY_DIR)/$(BINARY_NAME) \
		$(MAIN_GO_PATH)

build-debug:
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(BUILD_ARGS) \
		-race \
		$(LDFLAGS) \
		-o $(BINARY_DIR)/$(BINARY_NAME)-debug \
		$(MAIN_GO_PATH)

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f $(COVER_PROFILE) $(COVER_HTML)

# Run tests with coverage
test:
	CGO_ENABLED=1 $(GOTEST) -v -race -coverprofile=$(COVER_PROFILE) $(TEST_PKGS)

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
image:
	docker build -t \
		$(KEPLER_IMAGE) \
		--platform=linux/$(GOARCH) .
	$(call docker_tag,$(KEPLER_IMAGE),$(ADDITIONAL_TAGS))

# Push Docker image
push:
	$(call docker_push,$(KEPLER_IMAGE),$(ADDITIONAL_TAGS))

# Run the application
run:
	$(BINARY_DIR)/$(BINARY_NAME)

# docker_tag accepts an image:tag and a list of additional tags comma-separated
# it tags the image with the additional tags
# E.g. given foo:bar, a,b,c, it will tag foo:bar as foo:a, foo:b, foo:c
define docker_tag
@{ \
	set -eu ;\
	img="$(1)" ;\
	tags="$(2)" ;\
	echo "tagging container image $$img with additional tags: '$$tags'" ;\
	\
	img_path=$${img%:*} ;\
	for tag in $$(echo $$tags | tr -s , ' ' ); do \
		docker tag $$img $$img_path:$$tag ;\
	done \
}
endef

# docker_push accepts an image:tag and a list of additional tags comma-separated
# it pushes the image:tag and all other images with the additional tags
# E.g. given foo:bar, a,b,c, it will push foo:bar, foo:a, foo:b, foo:c
define docker_push
@{ \
	set -eu ;\
	img="$(1)" ;\
	tags="$(2)" ;\
	echo "docker push $$img and additional tags: '$$tags'" ;\
	\
	img_path=$${img%:*} ;\
	docker push $$img ;\
	for tag in $$(echo $$tags | tr -s , ' ' ); do \
		docker push $$img_path:$$tag ;\
	done \
}
endef
