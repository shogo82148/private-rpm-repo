Name:           mackerel
Version:        1.0.1
Release:        1%{?dist}
Summary:        the repository for mackerel.io

Group:          System Environment/Base
License:        MIT

URL:            https://github.com/shogo82148
Source0:        mackerel-rhel.repo
Source1:        mackerel-amzn2023.repo

BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)

BuildArch:     noarch

%description
This package contains mackerel.io repository configuration files.

%prep

%if 0%{?amzn}
cp %{SOURCE1} mackerel.repo
%else
cp %{SOURCE0} mackerel.repo
%endif

%build

%install
rm -rf $RPM_BUILD_ROOT

# yum
install -dm 755 $RPM_BUILD_ROOT%{_sysconfdir}/yum.repos.d
install -pm 644 mackerel.repo $RPM_BUILD_ROOT%{_sysconfdir}/yum.repos.d

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(-,root,root,-)
%config(noreplace) /etc/yum.repos.d/*

%changelog
* Tue Sep 22 2026 ICHINOSE Shogo <shogo82148@gmail.com> - 1.0.1-1
- Add support for Amazon Linux 2023, AlmaLinux 9 and 10, Rocky Linux 9 and 10
- Drop support for Amazon Linux 2

* Mon Mar 22 2021 Ichinose Shogo <shogo82148@gmail.com> - 1.0.0-1
- Create Package
