ARG ARCH=amd64

FROM quay.io/sustainable_computing_io/kepler_builder:ubi-8.6-bcc-0.24-go1.18 as builder


ARG MAKE_TARGET=cross-build-linux-$ARCH
ARG BIN_TIMESTAMP
ARG SOURCE_GIT_TAG

LABEL name=kepler-builder

ENV GOPATH=/opt/app-root GO111MODULE=on GOROOT=/usr/local/go 

ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH

WORKDIR $GOPATH/src/github.com/sustainable-computing-io/kepler


COPY . .
RUN mkdir -p data

# Build kepler
RUN make clean $MAKE_TARGET SOURCE_GIT_TAG=$SOURCE_GIT_TAG BIN_TIMESTAMP=$BIN_TIMESTAMP

# RUN make test

# build image
FROM quay.io/sustainable_computing_io/kepler_base:cuda-11.8.0-base-ubi8-bcc-0.24

ARG ARCH=amd64

COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/_output/bin/linux_$ARCH/kepler /usr/bin/kepler

RUN mkdir -p /var/lib/kepler/data
COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/data/normalized_cpu_arch.csv /var/lib/kepler/data/normalized_cpu_arch.csv

ADD https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentModelWeight/CgroupOnly/ScikitMixed/ScikitMixed.json /var/lib/kepler/data/ScikitMixed.json

ADD https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/AbsComponentModelWeight/Full/KerasCompWeightFullPipeline/KerasCompWeightFullPipeline.json /var/lib/kepler/data/KerasCompWeightFullPipeline.json

ENTRYPOINT ["/usr/bin/kepler"]
