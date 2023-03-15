%undefine _disable_source_fetch

Name:           container-kepler
Version:        %{getenv:_VERSION_}
Release:        %{getenv:_RELEASE_}
Summary:        Containerized Kepler

License:        Apache License 2.0
URL:            https://github.com/sustainable-computing-io/kepler/
Source0:        kepler.tar.gz

BuildRequires: systemd

Requires:       kernel-devel

%{?systemd_requires}

%description
Kubernetes-based Efficient Power Level Exporter

%build

%install

install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_sysconfdir}/kepler/

install -p -m644 ./packaging/rpm/container-kepler.service %{buildroot}%{_unitdir}/container-kepler.service


%post

%systemd_post container-kepler.service

%files
%license LICENSE
%{_unitdir}/container-kepler.service
