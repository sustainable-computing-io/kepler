ARG ARCH=amd64

FROM quay.io/sustainable_computing_io/tflite_builder:${ARCH} as tflite_builder

FROM quay.io/sustainable_computing_io/kepler_builder:go1.18 as builder


ARG MAKE_TARGET=cross-build-linux-$ARCH
ARG BIN_TIMESTAMP
ARG SOURCE_GIT_TAG

USER root

LABEL name=kepler-builder

ENV GOPATH=/opt/app-root GO111MODULE=on GOROOT=/usr/local/go 

ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH

WORKDIR $GOPATH/src/github.com/sustainable-computing-io/kepler

# Copy only neccessary components
COPY pkg pkg
COPY cmd cmd
COPY bpf_assets bpf_assets
COPY go.mod go.mod
COPY Makefile Makefile
COPY .git .git
RUN mkdir -p data

# Copy tflite-related dependencies from tflite builder
COPY --from=tflite_builder /tensorflow_src/bazel-bin/tensorflow/lite/libtensorflowlite.so /usr/local/lib/libtensorflowlite.so
COPY --from=tflite_builder /tensorflow_src/bazel-bin/tensorflow/lite/c/libtensorflowlite_c.so /usr/local/lib/libtensorflowlite_c.so

RUN mkdir -p lib/tensorflow/include/tensorflow/lite
COPY --from=tflite_builder /tensorflow_src/tensorflow/lite lib/tensorflow/include/tensorflow/lite

# Build kepler
RUN make clean $MAKE_TARGET SOURCE_GIT_TAG=$SOURCE_GIT_TAG BIN_TIMESTAMP=$BIN_TIMESTAMP

# Copy model data and test (TO-DO: move to CI)
COPY data/modeldb data/modeldb
COPY data/cpu_model.csv data/cpu_model.csv
COPY data/power_data.csv data/power_data.csv
RUN make test

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