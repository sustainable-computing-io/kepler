FROM registry.access.redhat.com/ubi8/ubi:8.4 as builder

USER root

LABEL name=kepler-builder

RUN yum update -y && \
    yum install -y  http://mirror.centos.org/centos/8-stream/PowerTools/x86_64/os/Packages/bcc-devel-0.19.0-4.el8.x86_64.rpm && \
    yum install -y kernel-devel make git gcc && \
    yum clean all -y 

RUN curl -LO https://go.dev/dl/go1.18.1.linux-amd64.tar.gz; mkdir -p /usr/local; tar -C /usr/local -xvzf go1.18.1.linux-amd64.tar.gz; rm -f go1.18.1.linux-amd64.tar.gz
