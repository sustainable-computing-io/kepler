all: kepler

##@ Help

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

### env define ###
export BIN_TIMESTAMP ?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
export TIMESTAMP ?=$(shell echo $(BIN_TIMESTAMP) | tr -d ':' | tr 'T' '-' | tr -d 'Z')

# restrict included verify-* targets to only process project files
SRC_ROOT           := $(shell pwd)
ARCH               := $(shell arch)
OUTPUT_DIR         := _output
CROSS_BUILD_BINDIR := $(OUTPUT_DIR)/bin
GIT_VERSION        := $(shell git describe --dirty --tags --always --match='v*')
GIT_SHA            := $(shell git rev-parse HEAD)
GIT_BRANCH         := $(shell git rev-parse --abbrev-ref HEAD)
VERSION            ?= $(GIT_VERSION)
ROOTLESS	       ?= false
IMAGE_REPO         ?= quay.io/sustainable_computing_io
BUILDER_IMAGE      ?= quay.io/sustainable_computing_io/kepler_builder:ubi-9-libbpf-1.2.0
IMAGE_NAME         ?= kepler
IMAGE_TAG          ?= latest
CTR_CMD            ?= $(or $(shell podman info > /dev/null 2>&1 && which podman), $(shell docker info > /dev/null 2>&1 && which docker))

# use CTR_CMD_PUSH_OPTIONS to add options to <container-runtime> push command.
# E.g. --tls-verify=false for local develop when using podman
CTR_CMD_PUSH_OPTIONS ?=

ifeq ($(DEBUG),true)
	# throw all the debug info in!
	LD_FLAGS =
	GC_FLAGS =-gcflags "all=-N -l"
else
	# strip everything we can
	LD_FLAGS =-w -s
	GC_FLAGS =
endif

GENERAL_TAGS := 'include_gcs include_oss containers_image_openpgp gssapi providerless netgo osusergo libbpf '
GPU_TAGS := ' gpu '
HABANA := false
ifeq ($(shell ldconfig -p | grep -q libnvml_injection.so && echo exists),exists)
	GPU_TAGS := ' nvml '
endif
ifeq ($(shell ldconfig -p | grep -q libdcgm.so && echo exists),exists)
	GPU_TAGS := ' dcgm '
endif
ifeq ($(shell ldconfig -p | grep -q libhlml.so && echo exists),exists)
	GPU_TAGS := ' habana '
	HABANA = true
endif

# set GOENV
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
GOENV := GOOS=$(GOOS) GOARCH=$(GOARCH)

LDFLAGS := $(LDFLAGS) \
		-X main.Version=$(VERSION) \
		-X main.Revision=$(GIT_SHA) \
		-X main.Branch=$(GIT_BRANCH) \
		-X main.OS=$(GOOS) \
		-X main.Arch=$(GOARCH)

GO_LD_FLAGS := $(GC_FLAGS) -ldflags "-X $(LD_FLAGS)" $(CFLAGS)

LIBBPF_HEADERS := /usr/include/bpf
GOENV = GO111MODULE="" GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 CC=clang CGO_CFLAGS="-I $(LIBBPF_HEADERS) -I/usr/include/" CGO_LDFLAGS="-lelf -lz -lbpf"

DOCKERFILE := $(SRC_ROOT)/build/Dockerfile
IMAGE_BUILD_TAG := $(GIT_VERSION)-linux-$(GOARCH)
GO_BUILD_TAGS := $(GENERAL_TAGS)$(GOOS)$(GPU_TAGS)
GO_TEST_TAGS := $(GENERAL_TAGS)$(GOOS)

# for testsuite
ENVTEST_ASSETS_DIR=$(SRC_ROOT)/test-bin
export PATH := $(PATH):$(SRC_ROOT)/test-bin

ifndef GOPATH
	GOPATH := $(HOME)/go
endif

ifndef GOBIN
	GOBIN := $(GOPATH)/bin
endif

# NOTE: project related tools get installed to tmp dir which is ignored by
PROJECT_DIR := $(shell dirname $(abspath $(firstword $(MAKEFILE_LIST))))
TOOLS_DIR=$(PROJECT_DIR)/tmp/bin
KUSTOMIZE = $(TOOLS_DIR)/kustomize
GOVULNCHECK = $(TOOLS_DIR)/govulncheck

base_dir := $(patsubst %/,%,$(dir $(realpath $(firstword $(MAKEFILE_LIST)))))

##@ Default
kepler: build_containerized ## Build Kepler.
.PHONY: kepler

clean: clean-cross-build
.PHONY: clean

##@ Container build.
image_builder_check:
	@if [ -z '$(CTR_CMD)' ] ; then echo '!! ERROR: containerized builds require podman||docker CLI, none found $$PATH' >&2 && exit 1; fi

build_image: image_builder_check ## Build image without DCGM.
	# build kepler without dcgm
	$(CTR_CMD) build -t $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_BUILD_TAG) \
		-f $(DOCKERFILE) \
		--build-arg INSTALL_DCGM=false \
		--build-arg INSTALL_HABANA=false \
		--build-arg VERSION=$(VERSION) \
		--platform="linux/$(GOARCH)" \
		.
	$(CTR_CMD) tag $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_BUILD_TAG) $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)
.PHONY: build_image

build_image_dcgm:  image_builder_check ## Build image with DCGM.
	# build kepler with dcgm
	$(CTR_CMD) build -t $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_BUILD_TAG)-"dcgm" \
		-f $(DOCKERFILE) \
		--build-arg INSTALL_DCGM=true \
		--build-arg INSTALL_HABANA=false \
		--build-arg VERSION=$(VERSION) \
		--platform="linux/$(GOARCH)" \
		.
	$(CTR_CMD) tag $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_BUILD_TAG)-dcgm $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)-dcgm

build_image_habana: image_builder_check ## Build image with Habana.
	# build kepler with habana
	$(CTR_CMD) build -t $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_BUILD_TAG)-"habana" \
		-f $(DOCKERFILE) \
		--build-arg INSTALL_HABANA=true \
		--build-arg INSTALL_DCGM=false \
		--build-arg VERSION=$(VERSION) \
		--platform="linux/$(GOARCH)" \
		.
	$(CTR_CMD) tag $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_BUILD_TAG)-habana $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)-habana
.PHONY: build_image_dcgm

build_containerized: build_image build_image_dcgm  build_image_habana## Build ALL container images.
.PHONY: build_containerized

save-image: ## Save container image.
	@mkdir -p _output
	$(CTR_CMD) save $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG) | gzip > "${IMAGE_OUTPUT_PATH}"
.PHONY: save-image

load-image: ## Load container image.
	$(CTR_CMD) load -i "${INPUT_PATH}"
.PHONY: load-image

image-prune: ## Image prune.
	$(CTR_CMD) image prune -a -f || true
.PHONY: image-prune

push-image:  ## Push image.
	$(CTR_CMD) push $(CTR_CMD_PUSH_OPTIONS) $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)
.PHONY: push-image

##@ General build
clean-cross-build:
	$(RM) -r '$(CROSS_BUILD_BINDIR)'
	$(RM) -rf $(OUTPUT_DIR)/staging
	if [ -d '$(OUTPUT_DIR)' ]; then rmdir --ignore-fail-on-non-empty '$(OUTPUT_DIR)'; fi
.PHONY: clean-cross-build

build: clean_build_local _build_local copy_build_local ##  Build binary and copy to $(OUTPUT_DIR)/bin
.PHONY: build

_build_ebpf_local:
	@make -C bpfassets/libbpf

_build_local: _build_ebpf_local ##  Build Kepler binary locally.
	@echo TAGS=$(GO_BUILD_TAGS)
	@mkdir -p "$(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)"
	+@$(GOENV) go build \
		-v -tags ${GO_BUILD_TAGS} \
		-ldflags "$(LDFLAGS)" \
		-o $(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)/kepler \
		./cmd/exporter/exporter.go

container_build: ## Run a container and build Kepler inside it.
	$(CTR_CMD) run --rm \
		-v $(base_dir):/kepler:Z -w /kepler \
		--user $(shell id -u):$(shell id -g) \
		-e GOROOT=/usr/local/go \
		-e PATH=/usr/bin:/bin:/sbin:/usr/local/bin:/usr/local/go/bin \
		$(BUILDER_IMAGE) \
		git config --add safe.directory /kepler && make build

##@ RPM build
build_rpm: ## Build the Kepler Binary RPM.
	rpmbuild packaging/rpm/kepler.spec --build-in-place -bb

build_container_rpm: ## Build the Containerized Kepler RPM.
	rpmbuild packaging/rpm/container-kepler.spec --build-in-place -bb

containerized_build_rpm: ## Build the Kepler Binary RPM inside a container.
	@mkdir -p $(base_dir)/$(OUTPUT_DIR)/rpmbuild
	$(CTR_CMD) run --rm \
		-v $(base_dir):/kepler:Z -w /kepler -v $(base_dir)/$(OUTPUT_DIR)/rpmbuild:/opt/app-root/src/rpmbuild \
		-e _VERSION_=${_VERSION_} -e _RELEASE_=${_RELEASE_} -e _ARCH_=${_ARCH_} \
		-e _TIMESTAMP_="$(shell date +"%a %b %d %Y")" -e _COMMITTER_=${_COMMITTER_} \
		-e PATH=$(PATH):/usr/local/go/bin \
		$(BUILDER_IMAGE) \
		make build_rpm

containerized_build_container_rpm: ## Build the Containerized Kepler RPM inside a container.
	@mkdir -p $(base_dir)/$(OUTPUT_DIR)/rpmbuild
	$(CTR_CMD) run --rm \
		-v $(base_dir):/kepler:Z -w /kepler -v $(base_dir)/$(OUTPUT_DIR)/rpmbuild:/opt/app-root/src/rpmbuild \
		-e _VERSION_=${_VERSION_} -e _RELEASE_=${_RELEASE_} \
		$(BUILDER_IMAGE) \
		make build_container_rpm

clean_build_local:
	rm -rf $(CROSS_BUILD_BINDIR)

copy_build_local:
	cp $(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)/kepler $(CROSS_BUILD_BINDIR)

cross-build-linux-amd64:
	+$(MAKE) _build_local GOOS=linux GOARCH=amd64
.PHONY: cross-build-linux-amd64

cross-build-linux-arm64:
	+$(MAKE) _build_local GOOS=linux GOARCH=arm64
.PHONY: cross-build-linux-arm64

cross-build-linux-s390x:
	+$(MAKE) _build_local GOOS=linux GOARCH=s390x
.PHONY: cross-build-linux-s390x

cross-build: clean_build_local cross-build-linux-amd64 cross-build-linux-arm64 cross-build-linux-s390x copy_build_local ## Build Kepler for multiple archs.
.PHONY: cross-build

## toolkit ###
.PHONY: tidy-vendor
tidy-vendor:
	go mod tidy -v
	go mod vendor
ifeq ($(HABANA), false)
	@echo "Tidy hlml.go"
	sed -i 's/cgo LDFLAGS/cgo habana LDFLAGS/g' vendor/github.com/HabanaAI/gohlml/hlml.go
endif

.PHONY: ginkgo-set
ginkgo-set:
	mkdir -p $(GOBIN)
	mkdir -p $(ENVTEST_ASSETS_DIR)
	@test -f $(ENVTEST_ASSETS_DIR)/ginkgo || \
	 (go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@v2.15.0  && \
	  cp $(GOBIN)/ginkgo $(ENVTEST_ASSETS_DIR)/ginkgo)

.PHONY: container_test
container_test:
	$(CTR_CMD) run --rm \
		-v $(base_dir):/kepler:Z\
		-v ~/.kube/config:/tmp/.kube/config \
		--network host \
		-w /kepler \
		--privileged \
		$(BUILDER_IMAGE) \
		/bin/sh -c ' \
			yum -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm && \
			yum install -y cpuid && \
			cd doc/ && \
			./dev/prepare_dev_env.sh && \
			cd - && git config --global --add safe.directory /kepler && \
			make VERBOSE=1 unit-test bench'

VERBOSE ?= 0
TMPDIR := $(shell mktemp -d)
TEST_PKGS := $(shell go list -tags $(GO_BUILD_TAGS) ./... | grep -v pkg/bpf | grep -v e2e)
SUDO?=sudo
SUDO_TEST_PKGS := $(shell go list -tags $(GO_BUILD_TAGS) ./... | grep pkg/bpf)

.PHONY: test
test: unit-test bpf-test bench ## Run all tests.

.PHONY: unit-test
unit-test: ginkgo-set tidy-vendor ## Run unit tests.
	@echo TAGS=$(GO_TEST_TAGS)
	$(if $(VERBOSE),@echo GOENV=$(GOENV))
	@$(GOENV) go test -tags $(GO_TEST_TAGS) \
		$(if $(VERBOSE),-v) \
		-cover -covermode=atomic -coverprofile=coverage.out \
		--race --count=1 \
		$(TEST_PKGS)

.PHONY: bench
bench: ## Run benchmarks.
	@echo TAGS=$(GO_TEST_TAGS)
	$(GOENV) go test -tags $(GO_TEST_TAGS) \
		$(if $(VERBOSE),-v) \
		-test.run=dontrunanytests \
		-bench=. --count=1 $(TEST_PKGS)

.PHONY: bpf-test
bpf-test: _build_ebpf_local ## Run BPF tests.
	for pkg in $(SUDO_TEST_PKGS); do \
		$(GOENV) go test -c $$pkg -tags $(GO_TEST_TAGS) -cover \
		-covermode=atomic -coverprofile=coverage.bpf.out \
		-o $(TMPDIR)/$$(basename $$pkg).test && \
		$(SUDO) $(TMPDIR)/$$(basename $$pkg).test; \
	done

.PHONY: test-mac-verbose
test-mac-verbose: ginkgo-set
	@echo TAGS=$(GO_TEST_TAGS)
	@go test \
		-covermode=atomic -coverprofile=coverage.out \
		--race --count=1 \
		$(TEST_PKGS)

escapes_detect: tidy-vendor
	@$(GOENV) go build -tags $(GO_BUILD_TAGS) -gcflags="-m -l" ./... 2>&1 | grep "escapes to heap" || true

check-govuln: govulncheck tidy-vendor
	@$(GOVULNCHECK) ./... || true

format:
	./automation/presubmit-tests/gofmt.sh

c-format:
	@echo "Checking c format"
	@git ls-files -- '*.c' '*.h' ':!:vendor' ':!:bpfassets/libbpf/include/' | xargs clang-format --dry-run --Werror

golint:
	@mkdir -p $(base_dir)/.cache/golangci-lint
	$(CTR_CMD) pull golangci/golangci-lint:latest
	$(CTR_CMD) run --tty --rm \
		--volume '$(base_dir)/.cache/golangci-lint:/root/.cache' \
		--volume '$(base_dir):/app' \
		--workdir /app \
		golangci/golangci-lint \
		golangci-lint run --verbose

TOOLS = govulncheck \
		jq \
		kubectl \
		kustomize \
		yq \

tools:
	./hack/tools.sh
.PHONY: tools

$(TOOLS):
	./hack/tools.sh $@

build-manifest: kustomize
	./hack/build-manifest.sh "${OPTS}"
.PHONY: build-manifest

##@ Development Env
CLUSTER_PROVIDER ?= kind
LOCAL_DEV_CLUSTER_VERSION ?= main

KIND_WORKER_NODES ?=2
BUILD_CONTAINERIZED ?= build_image

COMPOSE_DIR="$(PROJECT_DIR)/manifests/compose"
DEV_TARGET ?= dev

.PHONY: compose
compose: ## Setup kepler (latest) using docker compose
	docker compose \
			-f $(COMPOSE_DIR)/compose.yaml \
		up --build -d
	@echo -e "\nDeployment Overview (compose file: hack/compose.yaml) \n"
	@echo "Services"
	@echo "  * Grafana    : http://localhost:3000"
	@echo "  * Prometheus : http://localhost:9090"
	@echo -e "\nKepler Deployments"
	@echo "  * latest / upstream  : http://localhost:9288/metrics"

.PHONY: compose-clean
compose-clean: ## Cleanup kepler (latest) deployed using docker compose
	docker compose \
			-f $(COMPOSE_DIR)/compose.yaml \
		down --remove-orphans --volumes --rmi all

.PHONY: dev
dev: ## Setup development env using compose with 2 kepler (latest & current) deployed
	docker compose \
			-f $(COMPOSE_DIR)/$(DEV_TARGET)/compose.yaml \
		up --build -d
	@echo -e "\nDeployment Overview (compose file: hack/compose.yaml) \n"
	@echo "Services"
	@echo "  * Grafana    : http://localhost:3000"
	@echo "  * Prometheus : http://localhost:9090"
	@echo -e "\nKepler Deployments"
	@echo "  * development version : http://localhost:9188/metrics"
	@echo "  * latest / upstream   : http://localhost:9288/metrics"

dev-clean: ## Setup kepler (current and latest) along with
	docker compose \
			-f $(COMPOSE_DIR)/$(DEV_TARGET)/compose.yaml \
		down --remove-orphans --volumes --rmi all
.PHONY: dev-clean

dev-restart: dev-clean dev
.PHONY: dev-restart

cluster-clean: build-manifest ## Undeploy Kepler in the cluster.
	./hack/cluster-clean.sh
.PHONY: cluster-clean

cluster-deploy: ## Deploy Kepler in the cluster.
	BUILD_CONTAINERIZED=$(BUILD_CONTAINERIZED) \
	./hack/cluster-deploy.sh
.PHONY: cluster-deploy

cluster-up:  ## Create the Kind cluster, with Prometheus, Grafana and Kepler
	CLUSTER_PROVIDER=$(CLUSTER_PROVIDER) \
	LOCAL_DEV_CLUSTER_VERSION=$(LOCAL_DEV_CLUSTER_VERSION) \
	KIND_WORKER_NODES=$(KIND_WORKER_NODES) \
	BUILD_CONTAINERIZED=$(BUILD_CONTAINERIZED) \
	PROMETHEUS_ENABLE=true \
	GRAFANA_ENABLE=true \
	./hack/cluster.sh up
.PHONY: cluster-up

cluster-down: ## Delete the Kind cluster.
	CLUSTER_PROVIDER=$(CLUSTER_PROVIDER) \
	LOCAL_DEV_CLUSTER_VERSION=$(LOCAL_DEV_CLUSTER_VERSION) \
	KIND_WORKER_NODES=$(KIND_WORKER_NODES) \
	./hack/cluster.sh down
.PHONY: cluster-down

cluster-restart: ## Restart the Kind cluster.
	CLUSTER_PROVIDER=$(CLUSTER_PROVIDER) \
	LOCAL_DEV_CLUSTER_VERSION=$(LOCAL_DEV_CLUSTER_VERSION) \
	KIND_WORKER_NODES=$(KIND_WORKER_NODES) \
	./hack/cluster.sh restart
.PHONY: cluster-restart

e2e: # Run E2E integration tests.
	./hack/verify.sh integration
.PHONY: e2e

##@ platform-validation

VALIDATION_DOCKERFILE := $(SRC_ROOT)/build/Dockerfile.kepler-validator

build-validator: tidy-vendor format ## Build Validator.
	@echo TAGS=$(GO_BUILD_TAGS)
	@mkdir -p "$(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)"
	+@$(GOENV) go build -v -tags ${GO_BUILD_TAGS} -o $(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)/validator -ldflags "$(LDFLAGS)" ./cmd/validator/validator.go
	cp $(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)/validator $(CROSS_BUILD_BINDIR)
.PHONY: build-validator

build-validation-container: ## Build validation Container.
	$(CTR_CMD) build -t $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG) \
		-f $(VALIDATION_DOCKERFILE) .
.PHONY: build-validation-container

get-power:
	$(CTR_CMD) run -i --rm -v $(SRC_ROOT)/e2e/platform-validation:/output $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG) /usr/bin/validator
.PHONY: get-power

get-env:
	$(CTR_CMD) run -i --rm -v $(SRC_ROOT)/e2e/platform-validation:/output $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG) /usr/bin/validator -gen-env=true
.PHONY: get-env

platform-validation: ginkgo-set get-env ## Run Kepler platform validation.
	./hack/verify.sh platform
.PHONY: platform-validation

check: tidy-vendor check-govuln format golint test
.PHONY: check
