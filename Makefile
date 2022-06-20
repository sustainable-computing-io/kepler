export BIN_TIMESTAMP ?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
export TIMESTAMP ?=$(shell echo $(BIN_TIMESTAMP) | tr -d ':' | tr 'T' '-' | tr -d 'Z')

SOURCE_GIT_TAG :=$(shell git describe --tags --always --abbrev=7 --match 'v*')

SRC_ROOT :=$(shell pwd)

IMAGE_REPO :=quay.io/sustainable_computing_io/kepler
IMAGE_VERSION := "latest"
OUTPUT_DIR :=_output
CROSS_BUILD_BINDIR :=$(OUTPUT_DIR)/bin
FROM_SOURCE :=false
CTR_CMD :=$(or $(shell which podman 2>/dev/null), $(shell which docker 2>/dev/null))
ARCH :=$(shell uname -m |sed -e "s/x86_64/amd64/" |sed -e "s/aarch64/arm64/")

# restrict included verify-* targets to only process project files
GO_PACKAGES=$(go list ./cmd/... ./pkg/...)

ifeq ($(DEBUG),true)
	# throw all the debug info in!
	LD_FLAGS =
	GC_FLAGS =-gcflags "all=-N -l"
else
	# strip everything we can
	LD_FLAGS =-w -s
	GC_FLAGS =
endif

GO_LD_FLAGS := $(GC_FLAGS) -ldflags "-X $(LD_FLAGS)" $(CFLAGS)

GO_BUILD_FLAGS :=-tags 'include_gcs include_oss containers_image_openpgp gssapi providerless netgo osusergo'

OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)

kepler: build-containerized-cross-build-linux-amd64
.PHONY: kepler

tidy-vendor:
	go mod tidy
	go mod vendor


_build_local:
	@mkdir -p "$(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)"
	+@GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-o $(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)/kepler ./cmd/exporter.go

cross-build-linux-amd64:
	+$(MAKE) _build_local GOOS=linux GOARCH=amd64
.PHONY: cross-build-linux-amd64

cross-build-linux-arm64:
	+$(MAKE) _build_local GOOS=linux GOARCH=arm64
.PHONY: cross-build-linux-arm64

cross-build: cross-build-linux-amd64 cross-build-linux-arm64
.PHONY: cross-build


_build_containerized:
	@if [ -z '$(CTR_CMD)' ] ; then echo '!! ERROR: containerized builds require podman||docker CLI, none found $$PATH' >&2 && exit 1; fi
	echo BIN_TIMESTAMP==$(BIN_TIMESTAMP)
	$(CTR_CMD) build -t $(IMAGE_REPO):$(SOURCE_GIT_TAG)-linux-$(ARCH) \
		-f "$(SRC_ROOT)"/build/Dockerfile \
		--build-arg SOURCE_GIT_TAG=$(SOURCE_GIT_TAG) \
		--build-arg BIN_TIMESTAMP=$(BIN_TIMESTAMP) \
		--build-arg ARCH=$(ARCH) \
		--build-arg MAKE_TARGET="cross-build-linux-$(ARCH)" \
		--platform="linux/$(ARCH)" \
		.
	$(CTR_CMD) tag $(IMAGE_REPO):$(SOURCE_GIT_TAG)-linux-$(ARCH) $(IMAGE_REPO):$(IMAGE_VERSION)

.PHONY: _build_containerized

build-containerized-cross-build-linux-amd64:
	+$(MAKE) _build_containerized ARCH=amd64
.PHONY: build-containerized-cross-build-linux-amd64

build-containerized-cross-build-linux-arm64:
	+$(MAKE) _build_containerized ARCH=arm64
.PHONY: build-containerized-cross-build-linux-arm64

build-containerized-cross-build:
	+$(MAKE) build-containerized-cross-build-linux-amd64
	+$(MAKE) build-containerized-cross-build-linux-arm64
.PHONY: build-containerized-cross-build

# for testsuite
PWD=$(shell pwd)
ENVTEST_ASSETS_DIR=./test-bin
export PATH := $(PATH):./test-bin
GOPATH ?= $(HOME)/go

ginkgo-set:
	mkdir -p ${ENVTEST_ASSETS_DIR}
	@test -f $(ENVTEST_ASSETS_DIR)/ginkgo || \
	 (go get -u github.com/onsi/ginkgo/ginkgo && go install github.com/onsi/ginkgo/ginkgo && \
	  go get -u github.com/onsi/gomega/... && go install github.com/onsi/gomega/... && \
	  cp $(GOPATH)/bin/ginkgo $(ENVTEST_ASSETS_DIR)/ginkgo)
	
test: ginkgo-set tidy-vendor
	@go test -v ./...

clean-cross-build:
	$(RM) -r '$(CROSS_BUILD_BINDIR)'
	$(RM) -rf $(OUTPUT_DIR)/staging
	if [ -d '$(OUTPUT_DIR)' ]; then rmdir --ignore-fail-on-non-empty '$(OUTPUT_DIR)'; fi
.PHONY: clean-cross-build

clean: clean-cross-build
.PHONY: clean