FROM quay.io/sustainable_computing_io/kepler_base:ubi-8.6-bcc-0.24 as builder

USER root

LABEL name=kepler-builder

RUN yum install -y make git gcc libstdc++ make git gcc && \
    yum clean all -y 

ADD hack/golangInstall.sh .
RUN ./golangInstall.sh
# verify golang been installed
RUN /usr/local/go/bin/go version
