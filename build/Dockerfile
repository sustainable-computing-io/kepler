FROM quay.io/sustainable_computing_io/kepler_builder:latest as builder

ARG ARCH=amd64
ARG MAKE_TARGET=cross-build-linux-$ARCH
ARG BIN_TIMESTAMP
ARG SOURCE_GIT_TAG

USER root

LABEL name=kepler-builder

ENV GOPATH=/opt/app-root GOCACHE=/mnt/cache GO111MODULE=on

WORKDIR $GOPATH/src/github.com/sustainable-computing-io/kepler

COPY . .

RUN make clean $MAKE_TARGET SOURCE_GIT_TAG=$SOURCE_GIT_TAG BIN_TIMESTAMP=$BIN_TIMESTAMP

# build image
FROM quay.io/sustainable_computing_io/kepler_base:latest

ARG ARCH=amd64

COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/_output/bin/linux_$ARCH/kepler /usr/bin/kepler

RUN mkdir -p /var/lib/kepler/data
#ADD https://raw.githubusercontent.com/sustainable-computing-io/kepler/main/data/cpu_model.csv /var/lib/kepler/data/cpu_model.csv
#ADD https://raw.githubusercontent.com/sustainable-computing-io/kepler/main/data/power_data.csv /var/lib/kepler/data/power_data.csv
COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/data/cpu_model.csv /var/lib/kepler/data/cpu_model.csv
COPY --from=builder /opt/app-root/src/github.com/sustainable-computing-io/kepler/data/power_data.csv /var/lib/kepler/data/power_data.csv

ENTRYPOINT ["/usr/bin/kepler"]