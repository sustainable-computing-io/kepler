FROM quay.io/sustainable_computing_io/kepler_builder:ubi-9-bcc-0.26-go1.18 as builder

ARG BIN_TIMESTAMP
ARG SOURCE_GIT_TAG

LABEL name=kepler-builder

ENV GOPATH=/opt/app-root GO111MODULE=on GOROOT=/usr/local/go 

ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH

WORKDIR $GOPATH/src/github.com/sustainable-computing-io/kepler


COPY . .
RUN mkdir -p data

# Build kepler
RUN make build SOURCE_GIT_TAG=$SOURCE_GIT_TAG BIN_TIMESTAMP=$BIN_TIMESTAMP
RUN ls ./_output/bin
# RUN make test

# build image
FROM quay.io/sustainable_computing_io/kepler_base:ubi-9-bcc-0.26

COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/_output/bin/kepler /usr/bin/kepler

RUN mkdir -p /var/lib/kepler/data
COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/data/normalized_cpu_arch.csv /var/lib/kepler/data/normalized_cpu_arch.csv

ADD https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentModelWeight/CgroupOnly/ScikitMixed/ScikitMixed.json /var/lib/kepler/data/ScikitMixed.json

ADD https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/AbsComponentModelWeight/Full/KerasCompWeightFullPipeline/KerasCompWeightFullPipeline.json /var/lib/kepler/data/KerasCompWeightFullPipeline.json

# pre install kernel sources
RUN mkdir -p /usr/share/kepler/kernel_sources

COPY --from=quay.io/sustainable_computing_io/kepler_kernel_source_images:ubi8 /usr/src/kernels /usr/share/kepler/kernel_sources
COPY --from=quay.io/sustainable_computing_io/kepler_kernel_source_images:ubi9 /usr/src/kernels /usr/share/kepler/kernel_sources

ENTRYPOINT ["/usr/bin/kepler"]