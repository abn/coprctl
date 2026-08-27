Name:           coprctl
Version:        0.1.0
Release:        1%{?dist}
Summary:        Reimagined CLI and agent interface for Fedora Copr

License:        MIT
URL:            https://example.org/coprctl
Source0:        https://example.org/coprctl-%{version}.tar.gz

BuildRequires:  golang >= 1.26

%description
coprctl is a reimagined command-line and agent interface for the Fedora Copr
build system. It provides a coherent noun-verb command grammar, live build
logs, declarative project state, and a machine-readable agent interface.

%prep
%autosetup

%build
go build -trimpath -o bin/coprctl ./cmd/coprctl

%install
install -Dpm0755 bin/coprctl %{buildroot}%{_bindir}/coprctl

%files
%{_bindir}/coprctl

%changelog
* Thu Aug 27 2026 coprctl contributors - 0.1.0-1
- Initial release
