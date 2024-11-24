FROM quay.io/sustainable_computing_io/kepler_builder:ubi-9-libbpf-1.3.0 AS builder
ARG INSTALL_DCGM=false
ARG INSTALL_HABANA=false
WORKDIR /workspace

COPY . .

RUN if [[ "$INSTALL_DCGM" == "true" ]]; then \
		dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/rhel9/x86_64/cuda-rhel9.repo; \
		yum install -y datacenter-gpu-manager libnvidia-ml; \
	fi; \
	if [[ "$INSTALL_HABANA" == "true" ]]; then \
		rpm -Uvh https://vault.habana.ai/artifactory/rhel/9/9.2/habanalabs-firmware-tools-1.15.1-15.el9.x86_64.rpm --nodeps; \
		echo /usr/lib/habanalabs > /etc/ld.so.conf.d/habanalabs.conf; \
		ldconfig; \
	fi;

RUN make tidy-vendor format

RUN make build

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
# NOTE: use bash instead of sh
SHELL ["/bin/bash", "-c"]

ARG INSTALL_DCGM=false
ARG INSTALL_HABANA=false
ARG TARGETARCH

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility
ENV NVIDIA_MIG_MONITOR_DEVICES=all
ENV NVIDIA_MIG_CONFIG_DEVICES=all

RUN microdnf clean all && microdnf upgrade -y --refresh --best --nodocs --noplugins --setopt=install_weak_deps=0 && microdnf clean all
# NOTE: set -e ensure all fails results in a non-zero exit code (i.e. fail)
RUN set -e -x ;\
		INSTALL_PKGS=" \
			libbpf  \
		" ;\
		microdnf install -y $INSTALL_PKGS ;\
		\
		if [[ "$TARGETARCH" == "amd64" ]]; then \
			rpm -i https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm; \
			microdnf install -y cpuid; \
			if [[ "$INSTALL_DCGM" == "true" ]]; then \
				curl https://developer.download.nvidia.com/compute/cuda/repos/rhel9/x86_64/cuda-rhel9.repo -o /etc/yum.repos.d/cuda-rhel9.repo; \
				microdnf install -y datacenter-gpu-manager libnvidia-ml; \
			fi; \
			if [[ "$INSTALL_HABANA" == "true" ]]; then \
				rpm -Uvh https://vault.habana.ai/artifactory/rhel/9/9.2/habanalabs-firmware-tools-1.15.1-15.el9.x86_64.rpm --nodeps; \
				echo /usr/lib/habanalabs > /etc/ld.so.conf.d/habanalabs.conf; \
				ldconfig; \
			fi; \
		fi;\
		microdnf clean all

COPY --from=builder /workspace/_output/bin/kepler /usr/bin/kepler
COPY --from=builder /libbpf-source/linux-5.14.0-473.el9/tools/bpf/bpftool/bpftool /usr/bin/bpftool

RUN mkdir -p /var/lib/kepler/data
COPY --from=builder /workspace/data/cpus.yaml /var/lib/kepler/data/cpus.yaml

# copy model weight
COPY --from=builder /workspace/data/model_weight /var/lib/kepler/data/model_weight
ENTRYPOINT ["/usr/bin/kepler"]
