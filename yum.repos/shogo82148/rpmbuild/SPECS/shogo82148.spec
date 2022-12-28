%define repo_name rhel

%if 0%{?centos}
%define repo_name centos
%endif

%if 0%{?fedora}
%define repo_name fedora
%endif

%if 0%{?almalinux}
%define repo_name almalinux
%endif

%if 0%{?rocky}
%define repo_name rockylinux
%endif

%if 0%{?amzn}
%define repo_name amazonlinux
%endif

Name:           shogo82148
Version:        1.0.4
Release:        1%{?dist}
Summary:        shogo82148's Original Packages

Group:          System Environment/Base
License:        MIT

URL:            https://github.com/shogo82148
Source0:        RPM-GPG-KEY-shogo82148
Source1:        shogo82148.repo.in

BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)

BuildArch:     noarch

%description
This package contains shogo82148 (Ichinose Shogo) repository GPG key and configuration files.

%prep

cp %{SOURCE1} .
sed -e 's|__REPONAME__|'%{repo_name}'|g' < %{SOURCE1} > shogo82148.repo

%build

%install
rm -rf $RPM_BUILD_ROOT

#GPG Key
install -dm 755 $RPM_BUILD_ROOT%{_sysconfdir}/pki/rpm-gpg
install -pm 644 %{SOURCE0} $RPM_BUILD_ROOT%{_sysconfdir}/pki/rpm-gpg

# yum
install -dm 755 $RPM_BUILD_ROOT%{_sysconfdir}/yum.repos.d
install -pm 644 shogo82148.repo $RPM_BUILD_ROOT%{_sysconfdir}/yum.repos.d

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(-,root,root,-)
%config(noreplace) /etc/yum.repos.d/*
/etc/pki/rpm-gpg/*

%changelog
* Wed Dec 28 2022 ICHINOSE Shogo <shogo82148@gmail.com> - 1.0.4-1
- Fix minor version problem of Amazon Linux 2022

* Fri Jul 15 2022 ICHINOSE Shogo <shogo82148@gmail.com> - 1.0.4-1
- Add AlmaLinux9
- Add Amazon Linux 2022

* Wed Jun 01 2022 ICHINOSE Shogo <shogo82148@gmail.com> - 1.0.3-1
- Fix AlmaLinux9 RPM

* Mon May 16 2022 Ichinose Shogo <shogo82148@gmail.com> - 1.0.2-1
- Add AlmaLinux9
- Add Amazon Linux 2022

* Sun Apr 04 2021 Ichinose Shogo <shogo82148@gmail.com> - 1.0.1-1
- Support AlmaLinux 8

* Sun Feb 14 2021 Ichinose Shogo <shogo82148@gmail.com> - 1.0.0-1
- Create Package
