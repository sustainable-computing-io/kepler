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

CGO_ENABLED ?= 0

# Project parameters
BINARY_NAME=kepler

PRODUCTION ?= 0
ifeq ($(PRODUCTION), 1)
	# add -release suffix to binary name
	BINARY_NAME:=$(BINARY_NAME)-release
endif

BINARY_DIR=bin
MAIN_GO_PATH=./cmd/kepler
VERSION?=$(shell git describe --tags --always --dirty | sed 's/-reboot//' || echo "dev")
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
.PHONY: build
build:
	mkdir -p $(BINARY_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(BUILD_ARGS) \
		$(LDFLAGS) \
		-o $(BINARY_DIR)/$(BINARY_NAME) \
		$(MAIN_GO_PATH)

.PHONY: build-debug
build-debug:
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(BUILD_ARGS) \
		-race \
		$(LDFLAGS) \
		-o $(BINARY_DIR)/$(BINARY_NAME)-debug \
		$(MAIN_GO_PATH)

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f $(COVER_PROFILE) $(COVER_HTML)

# Run tests with coverage
.PHONY: test
test:
	CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -v -race -coverprofile=$(COVER_PROFILE) $(TEST_PKGS)

# Generate coverage report
.PHONY: coverage
coverage: test
	$(GOCMD) tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)

# Generate metrics documentation
.PHONY: gen-metrics-docs
gen-metrics-docs:
	$(GOCMD) run ./hack/gen-metric-docs/main.go --output metrics.md
	mv metrics.md docs/metrics/

# Run linting
.PHONY: lint
lint:
	$(GOLINT) run ./...

# Run go vet
.PHONY: vet
vet:
	$(GOVET) ./...

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) ./...

# Tidy and verify dependencies
.PHONY: deps
deps:
	$(GOMOD) tidy
	$(GOMOD) verify

# Build Docker image
.PHONY: image
image:
	docker build -t \
		$(KEPLER_IMAGE) \
		--platform=linux/$(GOARCH) .
	$(call docker_tag,$(KEPLER_IMAGE),$(ADDITIONAL_TAGS))

# Push Docker image
.PHONY: push
push:
	$(call docker_push,$(KEPLER_IMAGE),$(ADDITIONAL_TAGS))

# Run the application
.PHONY: run
run:
	$(BINARY_DIR)/$(BINARY_NAME)

# K8s Development env
CLUSTER_PROVIDER ?= kind
LOCAL_DEV_CLUSTER_VERSION ?= main
GRAFANA_ENABLE ?= false
PROMETHEUS_ENABLE ?= true
KIND_WORKER_NODES ?=2

# setup a cluster for local development
.PHONY: cluster-up
cluster-up:
	CLUSTER_PROVIDER=$(CLUSTER_PROVIDER) \
	VERSION=$(LOCAL_DEV_CLUSTER_VERSION) \
	GRAFANA_ENABLE=$(GRAFANA_ENABLE) \
	PROMETHEUS_ENABLE=$(PROMETHEUS_ENABLE) \
	KIND_WORKER_NODES=$(KIND_WORKER_NODES) \
	./hack/cluster.sh up

# restart the local development cluster
.PHONY: cluster-restart
cluster-restart:
	CLUSTER_PROVIDER=$(CLUSTER_PROVIDER) \
	VERSION=$(LOCAL_DEV_CLUSTER_VERSION) \
	GRAFANA_ENABLE=$(GRAFANA_ENABLE) \
	PROMETHEUS_ENABLE=$(PROMETHEUS_ENABLE) \
	KIND_WORKER_NODES=$(KIND_WORKER_NODES) \
	./hack/cluster.sh restart

# delete the local development cluster
.PHONY: cluster-down
cluster-down:
	CLUSTER_PROVIDER=$(CLUSTER_PROVIDER) \
	VERSION=$(LOCAL_DEV_CLUSTER_VERSION) \
	./hack/cluster.sh down

# Deploy Kepler to the K8s cluster
.PHONY: deploy
deploy:
	kubectl kustomize manifests/k8s | \
	sed -e "s|<KEPLER_IMAGE>|$(KEPLER_IMAGE)|g" | \
	kubectl apply --server-side --force-conflicts -f -

# Undeploy Kepler from the K8s cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
.PHONY: undeploy
undeploy:
	kubectl delete -k manifests/k8s --ignore-not-found=true

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
