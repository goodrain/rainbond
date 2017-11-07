Summary: acp-node
Name:    gr-acp-node
Version: %{_version}
Release: %{_release}
License: GPL
Group: goodrain
Source: gr-acp-node-%{version}.tar.gz
Packager: zhengys
BuildRoot: /root/rpmbuild

%description
acp-node

%prep
%setup -n gr-acp-node-%{version}

%build

%install
install -d %{buildroot}/usr/local/acp-node/
install -d %{buildroot}/usr/local/bin/
install -d %{buildroot}/usr/lib/systemd/system/
install -d %{buildroot}/usr/share/gr-acp-node/scripts/

install -p -m 755 usr/local/bin/acp-node %{buildroot}/usr/local/bin/acp-node
install -p -m 644 usr/lib/systemd/system/acp-node.service %{buildroot}/usr/lib/systemd/system/acp-node.service
install -p -m 755 usr/share/gr-acp-node/scripts/start-node.sh %{buildroot}/usr/share/gr-acp-node/scripts/start-node.sh
install -p -m 755 usr/local/acp-node/sh.tgz %{buildroot}/usr/local/acp-node/

%pre
[ -d "/etc/goodrain/envs" ] || mkdir -p /etc/goodrain/envs 
[ -f "/etc/goodrain/envs/acp-node.sh" ] && rm /etc/goodrain/envs/acp-node.sh
[ -f "/etc/goodrain/envs/ip.sh" ] && (
    grep "MANAGE" /etc/goodrain/envs/ip.sh 
    if [ $? -eq 0 ];then
        echo "NODE_TYPE=compute" >> /etc/goodrain/envs/acp-node.sh
    else
        echo "setting mode type master"
        echo "NODE_TYPE=" >> /etc/goodrain/envs/acp-node.sh
    fi
) || (
    echo "not init"
    exit 1
)


%post
%systemd_post acp-node
[ -L "/usr/bin/acp-node" ] || ln -s /usr/local/bin/acp-node /usr/bin/acp-node
[ -f "/usr/local/acp-node/sh.tgz" ] && (
    tar xf /usr/local/acp-node/sh.tgz -C /usr/local/acp-node/
    rm /usr/local/acp-node/sh.tgz
)

%preun
%systemd_preun acp-node

%postun
%systemd_postun_with_restart acp-node
[ -L "/usr/bin/acp-node" ] || rm -f /usr/bin/acp-node

%files
/usr/local/acp-node/
/usr/local/bin/acp-node
/usr/lib/systemd/system/acp-node.service
/usr/share/gr-acp-node/scripts/start-node.sh