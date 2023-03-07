Name:           kepler
Version:        standalone
Release:        0.4
Summary:        Kepler Binary

License:        Apache License 2.0
URL:            https://github.com/sustainable-computing-io/kepler/
Source0:        https://github.com/sustainable-computing-io/kepler/archive/refs/tags/standalone.tar.gz



BuildRequires: gcc
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

# golang specifics
%global golang_version 1.19

%global debug_package %{nil} 
%prep
%autosetup


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
install -p -m755 ./_output/kepler  %{buildroot}%{_bindir}/kepler
install -p -m644 ./packaging/systemd/kepler.service %{buildroot}%{_unitdir}/kepler.service


%post

%systemd_post kepler.service

%files
%license LICENSE
%{_bindir}/kepler
%{_unitdir}/kepler.service


%changelog
* Wed Feb 08 2023 Parul <parsingh@redhat.com>
- Initial packaging

