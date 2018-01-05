Summary: rainbond-node
Name:    gr-rainbond-node
Version: %{_version}
Release: %{_release}
License: GPL
Group: goodrain
Source: gr-rainbond-%{version}.tar.gz
Packager: ysicing
BuildRoot: /root/rpmbuild

%description
rainbond-node 

%prep
%setup -n gr-rainbond-%{version}

%build

%install
install -d %{buildroot}/usr/share/gr-rainbond-node/gaops/
install -d %{buildroot}/usr/local/bin/
install -d %{buildroot}/usr/lib/systemd/system/
install -d %{buildroot}/usr/share/gr-rainbond-node/scripts/

install -p -m 755 usr/local/bin/rainbond-node %{buildroot}/usr/local/bin/rainbond-node
install -p -m 644 usr/lib/systemd/system/rainbond-node.service %{buildroot}/usr/lib/systemd/system/rainbond-node.service
install -p -m 755 usr/share/gr-rainbond-node/scripts/start-node.sh %{buildroot}/usr/share/gr-rainbond-node/scripts/start-node.sh
install -p -m 755 usr/share/gr-rainbond-node/gaops/gaops.tgz %{buildroot}/usr/share/gr-rainbond-node/gaops/


%pre
[ -d "/etc/goodrain/envs" ] || mkdir -p /etc/goodrain/envs
[ -f "/etc/goodrain/envs/rainbond-node.sh" ] || (
    if [ -f "/etc/goodrain/envs/.role" ];then
        grep "manage" /etc/goodrain/envs/.role 
        if [ $? -eq 0 ];then
            echo "NODE_TYPE=" >> /etc/goodrain/envs/rainbond-node.sh
            echo "ROLE=$(cat /etc/goodrain/envs/.role | awk -F ':' '{print $2}')" >> /etc/goodrain/envs/rainbond-node.sh
        else
            echo "NODE_TYPE=compute" >> /etc/goodrain/envs/rainbond-node.sh
            echo "ROLE=$(cat /etc/goodrain/envs/.role | awk -F ':' '{print $2}')" >> /etc/goodrain/envs/rainbond-node.sh
        fi
    else
        echo "NODE_TYPE=" >> /etc/goodrain/envs/rainbond-node.sh
        echo "ROLE=manage,compute" >> /etc/goodrain/envs/rainbond-node.sh
    fi
)



%post
%systemd_post rainbond-node
[ -L "/usr/bin/rainbond-node" ] || ln -s /usr/local/bin/rainbond-node /usr/bin/rainbond-node
[ -f "/usr/share/gr-rainbond-node/gaops/gaops.tgz" ] && (
    tar xf /usr/share/gr-rainbond-node/gaops/gaops.tgz -C /usr/share/gr-rainbond-node/gaops/
)

%preun
%systemd_preun rainbond-node

%postun
%systemd_postun_with_restart rainbond-node
[ -L "/usr/bin/rainbond-node" ] || rm -f /usr/bin/rainbond-node

%files
/usr/share/gr-rainbond-node/gaops/
/usr/local/bin/rainbond-node
/usr/lib/systemd/system/rainbond-node.service
/usr/share/gr-rainbond-node/scripts/start-node.sh