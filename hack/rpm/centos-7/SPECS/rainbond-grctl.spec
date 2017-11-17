Summary: rainbond-grctl
Name:    gr-rainbond-grctl
Version: %{_version}
Release: %{_release}
License: GPL
Group: goodrain
Source: gr-rainbond-%{version}.tar.gz
Packager: ysicing
BuildRoot: /root/rpmbuild

%description
grctl, your best friend

%prep
%setup -n gr-rainbond-%{version}

%build

%install
install -d %{buildroot}/usr/local/bin/


install -p -m 755 usr/local/bin/rainbond-grctl %{buildroot}/usr/local/bin/rainbond-grctl


%pre

%post
%systemd_post rainbond-grctl
[ -L "/usr/bin/grctl" ] || ln -s /usr/local/bin/rainbond-grctl /usr/bin/grctl

%preun


%postun
%systemd_postun_with_restart rainbond-grctl
[ -L "/usr/bin/grctl" ] || rm -f /usr/bin/grctl

%files
/usr/local/bin/rainbond-grctl
