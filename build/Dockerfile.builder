FROM fedora:35 as builder

USER root

LABEL name=kepler-builder

# pick up latest bcc from fedora 35
RUN yum update -y && yum install -y bcc bcc-devel make golang && yum clean all -y 
