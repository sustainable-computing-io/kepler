%undefine _disable_source_fetch

Name:           kepler
Version:        %{getenv:_VERSION_}
Release:        %{getenv:_RELEASE_}
BuildArch:      %{getenv:_ARCH_}
Summary:        Kepler Binary

License:        Apache License 2.0
URL:            https://github.com/sustainable-computing-io/kepler/
Source0:        kepler.tar.gz

BuildRequires: systemd
BuildRequires: clang llvm llvm-devel zlib-devel make libbpf

Requires:       elfutils-libelf
Requires:       elfutils-libelf-devel

%{?systemd_requires}

%description
Kubernetes-based Efficient Power Level Exporter

%build
GOOS=linux
CROSS_BUILD_BINDIR=_output/bin

%ifarch x86_64
GOARCH=amd64
%define TARGETARCH amd64
%endif

%ifarch aarch64
GOARCH=arm64
%define TARGETARCH arm64
%endif

%ifarch s390x
GOARCH=s390
%define TARGETARCH s390
%endif

%define CHANGELOG "%( echo ../../CHANGELOG.md )"

make genlibbpf _build_local GOOS=${GOOS} GOARCH=${GOARCH} ATTACHER_TAG=libbpf

cp ./${CROSS_BUILD_BINDIR}/${GOOS}_${GOARCH}/kepler ./_output/kepler
echo -n "true" > ./_output/ENABLE_PROCESS_METRICS

%install

install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_sysconfdir}/kepler/

install -d %{buildroot}/var/lib/kepler/data
install -d %{buildroot}/var/lib/kepler/bpfassets
install -d %{buildroot}/etc/kepler/kepler.config

install -p -m755 ./_output/kepler  %{buildroot}%{_bindir}/kepler
install -p -m644 ./packaging/rpm/kepler.service %{buildroot}%{_unitdir}/kepler.service
install -p -m644 ./bpfassets/libbpf/bpf.o/%{TARGETARCH}_kepler.bpf.o %{buildroot}/var/lib/kepler/bpfassets/%{TARGETARCH}_kepler.bpf.o
install -p -m644 ./_output/ENABLE_PROCESS_METRICS %{buildroot}/etc/kepler/kepler.config/ENABLE_PROCESS_METRICS
install -p -m644 ./data/cpus.yaml %{buildroot}/var/lib/kepler/data/cpus.yaml
install -p -m644 ./data/model_weight/acpi_AbsPowerModel.json %{buildroot}/var/lib/kepler/data/acpi_AbsPowerModel.json
install -p -m644 ./data/model_weight/acpi_DynPowerModel.json %{buildroot}/var/lib/kepler/data/acpi_DynPowerModel.json
install -p -m644 ./data/model_weight/intel_rapl_AbsPowerModel.json %{buildroot}/var/lib/kepler/data/intel_rapl_AbsPowerModel.json
install -p -m644 ./data/model_weight/intel_rapl_DynPowerModel.json %{buildroot}/var/lib/kepler/data/intel_rapl_DynPowerModel.json

%post

%systemd_post kepler.service

%files
%license LICENSE
%{_bindir}/kepler
%{_unitdir}/kepler.service
/var/lib/kepler/bpfassets/%{TARGETARCH}_kepler.bpf.o
/var/lib/kepler/data/cpus.yaml
/var/lib/kepler/data/acpi_AbsPowerModel.json
/var/lib/kepler/data/acpi_DynPowerModel.json
/var/lib/kepler/data/intel_rapl_AbsPowerModel.json
/var/lib/kepler/data/intel_rapl_DynPowerModel.json
/etc/kepler/kepler.config/ENABLE_PROCESS_METRICS

%changelog
* %{getenv:_TIMESTAMP_} %{getenv:_COMMITTER_}
%{CHANGELOG}