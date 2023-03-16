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

make _build_local GOOS=${GOOS} GOARCH=${GOARCH}

cp ./${CROSS_BUILD_BINDIR}/${GOOS}_${GOARCH}/kepler ./_output/kepler

%install

install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_sysconfdir}/kepler/

install -p -m755 ./_output/kepler  %{buildroot}%{_bindir}/kepler
install -p -m644 ./packaging/rpm/kepler.service %{buildroot}%{_unitdir}/kepler.service


%post

%systemd_post kepler.service

%files
%license LICENSE
%{_bindir}/kepler
%{_unitdir}/kepler.service


%changelog
* %{getenv:_TIMESTAMP_} %{getenv:_COMMITTER_} 
- %{getenv:_CHANGELOG_}
