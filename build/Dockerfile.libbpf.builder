#
# This Dockerfile is used for building the image `quay.io/sustainable_computing_io/kepler_builder:ubi-9-libbpf-0.5-go1.18`
#
FROM quay.io/sustainable_computing_io/kepler_base:ubi-9-libbpf-0.5 as builder

#USER root

LABEL name=kepler-builder

RUN yum install -y make git gcc rpm-build systemd && \
    yum clean all -y 

RUN curl -LO https://go.dev/dl/go1.18.10.linux-amd64.tar.gz; mkdir -p /usr/local; tar -C /usr/local -xvzf go1.18.10.linux-amd64.tar.gz; rm -f go1.18.10.linux-amd64.tar.gz
