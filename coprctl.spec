Name:           coprctl
%global debug_package %{nil}
# x-release-please-start-version
Version:        1.0.1
# x-release-please-end
Release:        2%{?dist}
Summary:        Reimagined CLI and agent interface for Fedora Copr

License:        MIT
URL:            https://coprctl.lab.abn.is/
Source0:        https://github.com/abn/coprctl/archive/refs/tags/v%{version}/coprctl-%{version}.tar.gz

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

# Shell completions, generated from the freshly built binary into the
# distribution-standard locations so bash, zsh, and fish pick them up with no
# per-user setup.
mkdir -p \
  %{buildroot}%{_datadir}/bash-completion/completions \
  %{buildroot}%{_datadir}/zsh/site-functions \
  %{buildroot}%{_datadir}/fish/vendor_completions.d
bin/coprctl completion bash > %{buildroot}%{_datadir}/bash-completion/completions/coprctl
bin/coprctl completion zsh > %{buildroot}%{_datadir}/zsh/site-functions/_coprctl
bin/coprctl completion fish > %{buildroot}%{_datadir}/fish/vendor_completions.d/coprctl.fish
chmod 0644 \
  %{buildroot}%{_datadir}/bash-completion/completions/coprctl \
  %{buildroot}%{_datadir}/zsh/site-functions/_coprctl \
  %{buildroot}%{_datadir}/fish/vendor_completions.d/coprctl.fish

%files
%{_bindir}/coprctl
%{_datadir}/bash-completion/completions/coprctl
%{_datadir}/zsh/site-functions/_coprctl
%{_datadir}/fish/vendor_completions.d/coprctl.fish

%changelog
* Tue Sep 01 2026 coprctl contributors - 1.0.0-2
- Install shell completions for bash, zsh, and fish
