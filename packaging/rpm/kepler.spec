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
BuildRequires: make

Requires:       cpuid
Requires:       kmod
Requires:       xz
Requires:       python3  
Requires:       bcc-devel
Requires:       bcc

%{?systemd_requires}

%description
Kubernetes-based Efficient Power Level Exporter

%build
GOOS=linux
CROSS_BUILD_BINDIR=_output/bin

%ifarch x86_64
GOARCH=amd64
%endif

make _build_local GOOS=${GOOS} GOARCH=${GOARCH} ATTACHER_TAG=libbpf

cp ./${CROSS_BUILD_BINDIR}/${GOOS}_${GOARCH}/kepler ./_output/kepler

%install

install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_sysconfdir}/kepler/

install -d %{buildroot}/var/lib/kepler/data

install -p -m755 ./_output/kepler  %{buildroot}%{_bindir}/kepler
install -p -m644 ./packaging/rpm/kepler.service %{buildroot}%{_unitdir}/kepler.service
install -p -m644 ./data/normalized_cpu_arch.csv %{buildroot}/var/lib/kepler/data/normalized_cpu_arch.csv
install -p -m644 ./data/model_weight/acpi_AbsPowerModel.json %{buildroot}/var/lib/kepler/data/acpi_AbsPowerModel.json
install -p -m644 ./data/model_weight/acpi_DynPowerModel.json %{buildroot}/var/lib/kepler/data/acpi_DynPowerModel.json
install -p -m644 ./data/model_weight/rapl_AbsPowerModel.json %{buildroot}/var/lib/kepler/data/rapl_AbsPowerModel.json
install -p -m644 ./data/model_weight/rapl_DynPowerModel.json %{buildroot}/var/lib/kepler/data/rapl_DynPowerModel.json

%post

%systemd_post kepler.service

%files
%license LICENSE
%{_bindir}/kepler
%{_unitdir}/kepler.service
/var/lib/kepler/data/normalized_cpu_arch.csv
/var/lib/kepler/data/acpi_AbsPowerModel.json
/var/lib/kepler/data/acpi_DynPowerModel.json
/var/lib/kepler/data/rapl_AbsPowerModel.json
/var/lib/kepler/data/rapl_DynPowerModel.json

%changelog
* %{getenv:_TIMESTAMP_} %{getenv:_COMMITTER_} 
- %{getenv:_CHANGELOG_}
