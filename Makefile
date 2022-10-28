export BIN_TIMESTAMP ?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
export TIMESTAMP ?=$(shell echo $(BIN_TIMESTAMP) | tr -d ':' | tr 'T' '-' | tr -d 'Z')

SOURCE_GIT_TAG :=$(shell git describe --tags --always --abbrev=7 --match 'v*')

SRC_ROOT :=$(shell pwd)

OUTPUT_DIR :=_output
CROSS_BUILD_BINDIR :=$(OUTPUT_DIR)/bin
FROM_SOURCE :=false
ARCH :=$(shell uname -m |sed -e "s/x86_64/amd64/" |sed -e "s/aarch64/arm64/")
GIT_VERSION     := $(shell git describe --dirty --tags --match='v*')
VERSION         ?= $(GIT_VERSION)
LDFLAGS         := "-w -s -X 'github.com/sustainable-computing-io/kepler/pkg/version.Version=$(VERSION)'"

ifdef IMAGE_REPO
	IMAGE_REPO := $(IMAGE_REPO)
else
	IMAGE_REPO := quay.io/sustainable_computing_io/kepler
endif

ifdef IMAGE_TAG
	IMAGE_TAG := $(IMAGE_TAG)
else
	IMAGE_TAG := latest
endif

ifdef CTR_CMD
	CTR_CMD := $(CTR_CMD)
else 
	CTR_CMD :=$(or $(shell which podman 2>/dev/null), $(shell which docker 2>/dev/null))
endif

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

GO_BUILD_TAGS := 'include_gcs include_oss containers_image_openpgp gssapi providerless netgo osusergo gpu'
ifneq ($(shell command -v ldconfig),)
  ifneq ($(shell ldconfig -p|grep bcc),)
     GO_BUILD_TAGS = 'include_gcs include_oss containers_image_openpgp gssapi providerless netgo osusergo gpu bcc'
  endif
endif

OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)

kepler: build-containerized-cross-build-linux-amd64
.PHONY: kepler

tidy-vendor:
	go mod tidy
	go mod vendor


_build_local: format
	@mkdir -p "$(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)"
	+@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -tags ${GO_BUILD_TAGS} \
		-o $(CROSS_BUILD_BINDIR)/$(GOOS)_$(GOARCH)/kepler -ldflags $(LDFLAGS) ./cmd/exporter.go

cross-build-linux-amd64:
	+$(MAKE) _build_local GOOS=linux GOARCH=amd64
.PHONY: cross-build-linux-amd64

cross-build-linux-arm64:
	+$(MAKE) _build_local GOOS=linux GOARCH=arm64
.PHONY: cross-build-linux-arm64

cross-build: cross-build-linux-amd64 cross-build-linux-arm64
.PHONY: cross-build


_build_containerized: format
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
	$(CTR_CMD) tag $(IMAGE_REPO):$(SOURCE_GIT_TAG)-linux-$(ARCH) $(IMAGE_REPO):$(IMAGE_TAG)

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

push-image:
	$(CTR_CMD) push $(IMAGE_REPO):$(IMAGE_TAG)
.PHONY: push-image

multi-arch-image-base:	
	$(CTR_CMD) pull --platform=linux/s390x quay.io/sustainable_computing_io/kepler_base:latest-s390x; \
	$(CTR_CMD) pull --platform=linux/amd64 quay.io/sustainable_computing_io/kepler_base:latest-amd64; \
	$(CTR_CMD) manifest create quay.io/sustainable_computing_io/kepler_base:latest quay.io/sustainable_computing_io/kepler_base:latest-s390x quay.io/sustainable_computing_io/kepler_base:latest-amd64 quay.io/sustainable_computing_io/kepler_base:latest-arm64; \
	$(CTR_CMD) manifest annotate --arch s390x quay.io/sustainable_computing_io/kepler_base:latest quay.io/sustainable_computing_io/kepler_base:latest-s390x; \
	$(CTR_CMD) push quay.io/sustainable_computing_io/kepler_base:latest
.PHONY: multi-arch-image-base

# for testsuite
PWD=$(shell pwd)
ENVTEST_ASSETS_DIR=./test-bin
export PATH := $(PATH):./test-bin

ifndef GOPATH
  GOPATH := $(HOME)/go
  GOBIN := $(GOPATH)/bin
endif

ginkgo-set: tidy-vendor
	mkdir -p $(GOBIN)
	mkdir -p ${ENVTEST_ASSETS_DIR}
	@test -f $(ENVTEST_ASSETS_DIR)/ginkgo || \
	 (go install github.com/onsi/ginkgo/ginkgo@v1.16.5  && \
	  cp $(GOBIN)/ginkgo $(ENVTEST_ASSETS_DIR)/ginkgo)
	
test: ginkgo-set tidy-vendor
	@go test -tags $(GO_BUILD_TAGS) ./... --race --bench=. -cover --count=1 --vet=all

test-verbose: ginkgo-set tidy-vendor
	@go test -tags $(GO_BUILD_TAGS) -covermode=atomic -coverprofile=coverage.out -v ./... --race --bench=. -cover --count=1 --vet=all

test-mac-verbose: tidy-vendor
	@go test ./... --race --bench=. -cover --count=1 --vet=all

escapes_detect: tidy-vendor
	@go build -tags $(GO_BUILD_TAGS) -gcflags="-m -l" ./... | grep "escapes to heap" || true

set_govulncheck:
	@go install golang.org/x/vuln/cmd/govulncheck@latest

govulncheck: set_govulncheck tidy-vendor
	@govulncheck -v ./... || true

format:
	gofmt -e -d -s -l -w pkg/ cmd/

golint:
	./hack/golint.sh


clean-cross-build:
	$(RM) -r '$(CROSS_BUILD_BINDIR)'
	$(RM) -rf $(OUTPUT_DIR)/staging
	if [ -d '$(OUTPUT_DIR)' ]; then rmdir --ignore-fail-on-non-empty '$(OUTPUT_DIR)'; fi
.PHONY: clean-cross-build

clean: clean-cross-build
.PHONY: clean

build-manifest:
	./hack/build-manifest.sh
.PHONY: build-manifest

cluster-clean: build-manifest
	./hack/cluster-clean.sh
.PHONY: cluster-clean

cluster-deploy: cluster-clean
	BARE_METAL_NODE_ONLY=false ./hack/cluster-deploy.sh
.PHONY: cluster-deploy

cluster-sync:
	./hack/cluster-sync.sh
.PHONY: cluster-sync

cluster-up:
	./hack/cluster-up.sh
.PHONY: cluster-up

cluster-down:
	./hack/cluster-down.sh
.PHONY: cluster-down
