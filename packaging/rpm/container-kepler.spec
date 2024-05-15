%undefine _disable_source_fetch

Name:           container-kepler
Version:        %{getenv:_VERSION_}
Release:        %{getenv:_RELEASE_}
Summary:        Containerized Kepler

License:        GPLv2+ and Apache-2.0 and BSD
URL:            https://github.com/sustainable-computing-io/kepler/
Source0:        kepler.tar.gz

BuildArch:  noarch

Requires: podman

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
%license LICENSE-APACHE
%license LICENSE-BSD2
%license LICENSE-GPL2

%{_unitdir}/container-kepler.service
