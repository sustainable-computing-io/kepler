ARG ARCH=amd64

FROM quay.io/sustainable_computing_io/kepler_builder:ubi-8.6-bcc-0.24-go1.18 as builder


ARG MAKE_TARGET=cross-build-linux-$ARCH
ARG BIN_TIMESTAMP
ARG SOURCE_GIT_TAG

USER root

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
FROM quay.io/sustainable_computing_io/kepler_base:ubi-8.6-bcc-0.24

ARG ARCH=amd64

COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/_output/bin/linux_$ARCH/kepler /usr/bin/kepler

RUN mkdir -p /var/lib/kepler/data
COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/data/normalized_cpu_arch.csv /var/lib/kepler/data/normalized_cpu_arch.csv
COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/data/power_data.csv /var/lib/kepler/data/power_data.csv
COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/data/power_model.csv /var/lib/kepler/data/power_model.csv

ENTRYPOINT ["/usr/bin/kepler"]