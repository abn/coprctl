Name:           coprctl
Version:        0.4.1 # x-release-please-version
Release:        1%{?dist}
Summary:        Reimagined CLI and agent interface for Fedora Copr

License:        MIT
URL:            https://coprctl.lab.abn.is/
Source0:        %{url}/archive/v%{version}/coprctl-%{version}.tar.gz

BuildRequires:  golang >= 1.26

%global commit 0000000

%description
coprctl is a reimagined command-line and agent interface for the Fedora Copr
build system. It provides a coherent noun-verb command grammar, live build
logs, declarative project state, and a machine-readable agent interface.

%prep
%autosetup

%build
export GOFLAGS=-mod=readonly
export CGO_ENABLED=0
go build -trimpath -o bin/coprctl \
  -ldflags "-s -w \
    -X github.com/abn/coprctl/internal/cli.version=%{version} \
    -X github.com/abn/coprctl/internal/cli.commit=%{commit} \
    -X github.com/abn/coprctl/internal/cli.date=%{SOURCE_DATE_EPOCH}" \
  ./cmd/coprctl

%install
install -Dpm0755 bin/coprctl %{buildroot}%{_bindir}/coprctl

%files
%{_bindir}/coprctl

%changelog
* Thu Aug 27 2026 coprctl contributors - 0.1.0-1
- Initial release
