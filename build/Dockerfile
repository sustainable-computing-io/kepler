FROM quay.io/sustainable_computing_io/kepler_builder:ubi-9-libbpf-1.2.0 as builder
ARG VERSION
WORKDIR /workspace

COPY . .

RUN make build VERSION=${VERSION}

FROM registry.access.redhat.com/ubi9:9.2
ARG INSTALL_DCGM
ARG INSTALL_DCGM=${INSTALL_DCGM:-""}
ARG TARGETARCH

RUN yum -y update

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility
ENV NVIDIA_MIG_CONFIG_DEVICES=all 
ENV NVIDIA_MIG_MONITOR_DEVICES=all 

RUN INSTALL_PKGS=" \
    libbpf \
    " && \
	yum install -y $INSTALL_PKGS 

RUN if [ "$TARGETARCH" == "amd64" ]; then \
        yum install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm; \
        yum install -y cpuid; \
        if [[ ! -z "$INSTALL_DCGM" ]]; then \
            dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/rhel9/x86_64/cuda-rhel9.repo; \
            yum install -y datacenter-gpu-manager; \
        fi; \
    fi

RUN yum clean all

COPY --from=builder /workspace/_output/bin/kepler /usr/bin/kepler
COPY --from=builder /libbpf-source/linux-5.14.0-333.el9/tools/bpf/bpftool/bpftool /usr/bin/bpftool
COPY --from=builder /libbpf-source/linux-5.14.0-333.el9/tools/bpf/bpftool/bpftool /usr/bin/bpftool

RUN mkdir -p /var/lib/kepler/data
RUN mkdir -p /var/lib/kepler/bpfassets
COPY --from=builder /workspace/data/cpus.yaml /var/lib/kepler/data/cpus.yaml
COPY --from=builder /workspace/bpfassets/libbpf/bpf.o /var/lib/kepler/bpfassets

# copy model weight
COPY --from=builder /workspace/data/model_weight/acpi_AbsPowerModel.json /var/lib/kepler/data/acpi_AbsPowerModel.json
COPY --from=builder /workspace/data/model_weight/acpi_DynPowerModel.json /var/lib/kepler/data/acpi_DynPowerModel.json
COPY --from=builder /workspace/data/model_weight/intel_rapl_AbsPowerModel.json /var/lib/kepler/data/intel_rapl_AbsPowerModel.json
COPY --from=builder /workspace/data/model_weight/intel_rapl_DynPowerModel.json /var/lib/kepler/data/intel_rapl_DynPowerModel.json
ENTRYPOINT ["/usr/bin/kepler"]
