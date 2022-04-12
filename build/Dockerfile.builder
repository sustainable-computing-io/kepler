FROM registry.access.redhat.com/ubi8/ubi:8.4 as builder

USER root

LABEL name=kepler-builder

RUN yum update -y && yum install -y  http://mirror.centos.org/centos/8-stream/PowerTools/x86_64/os/Packages/bcc-devel-0.19.0-4.el8.x86_64.rpm && \
    yum install -y kernel-devel make golang && yum clean all -y 
