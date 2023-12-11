FROM quay.io/sustainable_computing_io/kepler_builder:ubi-9-libbpf-1.2.0 as builder

WORKDIR /workspace

COPY . .

RUN ATTACHER_TAG=libbpf make build

FROM registry.access.redhat.com/ubi9-minimal:9.2
RUN microdnf -y update

ENV NVIDIA_VISIBLE_DEVICES=all

RUN INSTALL_PKGS=" \
    libbpf \
    " && \
	microdnf install -y $INSTALL_PKGS && \
	microdnf clean all

COPY --from=builder /workspace/_output/bin/kepler /usr/bin/kepler
COPY --from=builder /libbpf-source/linux-5.14.0-333.el9/tools/bpf/bpftool/bpftool /usr/bin/bpftool
COPY --from=builder /usr/bin/cpuid /usr/bin/cpuid

RUN mkdir -p /var/lib/kepler/data
RUN mkdir -p /var/lib/kepler/bpfassets
COPY --from=builder /workspace/data/cpus.yaml /var/lib/kepler/data/cpus.yaml
COPY --from=builder /workspace/bpfassets/libbpf/bpf.o /var/lib/kepler/bpfassets

# copy model weight
COPY --from=builder /workspace/data/model_weight/acpi_AbsPowerModel.json /var/lib/kepler/data/acpi_AbsPowerModel.json
COPY --from=builder /workspace/data/model_weight/acpi_DynPowerModel.json /var/lib/kepler/data/acpi_DynPowerModel.json
COPY --from=builder /workspace/data/model_weight/rapl_AbsPowerModel.json /var/lib/kepler/data/rapl_AbsPowerModel.json
COPY --from=builder /workspace/data/model_weight/rapl_DynPowerModel.json /var/lib/kepler/data/rapl_DynPowerModel.json
ENTRYPOINT ["/usr/bin/kepler"]
