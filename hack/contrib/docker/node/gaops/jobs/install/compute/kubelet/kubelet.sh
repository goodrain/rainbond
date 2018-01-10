#!/bin/bash


OS_VERSION=$1
DNS_SERVER=$2
HOST_IP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')
HOST_UUID=$(cat /etc/goodrain/host_uuid.conf | grep "host_uuid" | awk -F '=' '{print $2}')

export KUBE_SHARE_DIR="/grdata/services/k8s"

DNS=${DNS_SERVER%%,*}

log.info() {
  echo "       $*"
}

log.error() {
  echo " !!!     $*"
  echo ""
}

function log.stdout() {
    echo "$*" >&2
}

function proc::is_running() {
    proc=$1
    proc_info=$(status $proc 2>&1)
    proc_items=($proc_info)
    status=${proc_items[1]%/*}
    if [ "$status" == "start" ];then
        echo "$proc is running"
        return 0
    else
        echo "$proc is not running: <$proc_info>"
        return 1
    fi
}

function proc::stop() {
    proc=$1
    stop $proc
    return 0
}

function proc::start(){
    proc=$1
    start $proc
    return 0
}

function proc::restart(){
    proc=$1
    if [ "$RELEASE" == "ubuntu/trusty" ];then
        restart $proc
    else
        systemctl restart $proc
    fi
    return 0
}

function check_version() {
    grep "3.4" /etc/yum.repos.d/acp.repo
    if [ $? -eq 0 ];then
        kubelet_version=$(yum list | grep kubelet | awk -F '-' '{print $3}' | awk -F '.' '{print $1}')
        if [ $kubelet_version -ne "46" ];then
            sed -i "s/3.4/3.3/g" /etc/yum.repos.d/acp.repo
            yum clean all 
            yum makecache
        fi
    fi
}

function package::match(){
    str=$1
    egrep_compile=$2
    echo $str | egrep "$egrep_compile" >/dev/null
}

function package::is_installed() {
    log.info "check package install"
    pkgname=$1
    if [[ $OS_VERSION =~ "7" ]];then
        rpm -qi $pkgname >/dev/null
        if [ $? -gt 0 ];then
           log.info "package $pkgname is not installed"
            return 1
        else
            log.info "package $pkgname is already installed"
            return 0
        fi
    else
        pkginfo=$(apt search $pkgname 2>/dev/null | egrep "^$pkgname/")
        [ -z "pkginfo" ] && {
            log.info "can not find package:$pkgname"
            return 1
        }
        pkgitems=($pkginfo)
        pkglen=${#pkgitems[@]}
        [ $pkglen -lt 4 ] && {
            return 1
        }
        full_version=${pkgitems[1]}
        install_status=${pkgitems[3]}
        package::match $install_status "installed|upgradable" && {
            log.info "$pkgname is already installed"
            return 0
        } || {
            log.info "$pkgname is not installed"
            return 1
        }
    fi
}

function package::install() {
    pkgname=$1
    pkg_version=${2:-*}

    log.info "install $pkgname"
    if [[ $OS_VERSION =~ "7" ]];then
        #check_version
        yum install -y $pkgname
    else
        export HOST_IP=$HOST_IP
        export DNS_SERVERS=$DNS
        DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes -o Dpkg::Options::="--force-confold"  $package="$pkg_version"
    fi
}

function package::enable() {
    UNIT=$1
    if [[ "$OS_VERSION" =~ "7" ]];then
        systemctl is-enabled $UNIT || systemctl enable $UNIT
        systemctl is-active $UNIT || systemctl start $UNIT
        _EXIT=1
        for ((i=1;i<=3;i++ )); do
            sleep 1
            systemctl is-active $UNIT || systemctl start $UNIT && export _EXIT=0 && break
        done

        if [ $_EXIT -ne 0 ];then
            log.error "check failed. abort..."
            log.stdout '{
            "status":[ 
            { 
                "name":"start_kubelet", 
                "condition_type":"START_KUBELET", 
                "condition_status":"False"
            } 
            ], 
            "type":"install"
            }'
            exit $_EXIT
        fi
    else
        UUIT_U=$(echo "$UNIT" | awk -F '.' '{print $1}') # kubelet
        proc::is_running $UUIT_U || proc::start $UUIT_U
    fi
}

function sync_certificates() {
    mkdir -p /etc/goodrain/kubernetes
    [ -d "/etc/goodrain/envs" ] || mkdir -p /etc/goodrain/envs
    [ -f "/etc/goodrain/kubernetes/kubeconfig" ] && rm -rf /etc/goodrain/kubernetes/kubeconfig
    [ -f "/etc/goodrain/kubernetes/admin.kubeconfig" ] && rm -rf /etc/goodrain/kubernetes/admin.kubeconfig

    if [ -d "$KUBE_SHARE_DIR" ];then
        cp $KUBE_SHARE_DIR/*.kubeconfig /etc/goodrain/kubernetes/
        cp /etc/goodrain/kubernetes/admin.kubeconfig /etc/goodrain/kubernetes/kubeconfig
    fi
    #if [ -d "/grdata/kubernetes/" ];then
    #    cp /grdata/kubernetes/*.kubeconfig /etc/goodrain/kubernetes/
    #fi
}

function config_custom() {
    [ -f "/etc/goodrain/k8s/custom.conf" ] && (
        mv /etc/goodrain/k8s/custom.conf /etc/goodrain/k8s/custom.conf.bak
        cat >> /etc/goodrain/k8s/custom.conf << EOF
minport = 11000
maxport = 20000
etcdv3 = 127.0.0.1:2379
UUID_file                 = /etc/goodrain/host_uuid.conf
EOF
    )
}

function run() {
    sync_certificates
    package::is_installed gr-kubelet || (
        package::install gr-kubelet || (
            log.stdout '{
            "status":[ 
            { 
                "name":"install_kubelet", 
                "condition_type":"INSTALL_KUBELET", 
                "condition_status":"False"
            } 
            ],
            "type":"install"
            }'
            exit 1
        )
    )
    
    sed -i "s/register-node=true/register-node=false/g" /usr/share/gr-kubernetes/scripts/start-kubelet.sh
    
    if [ "$OS_VERSION" == "ubuntu/trusty" ];then
        log.info ""
    else
        if [ -f "/etc/goodrain/host_uuid.conf" ];then
            grep -q '^HOST_UUID' /etc/goodrain/envs/kubelet.sh || echo "HOST_UUID=$HOST_UUID" >> /etc/goodrain/envs/kubelet.sh
            sed -i "s/--hostname_override=\$HOST_IP/--hostname_override=\$HOST_UUID/g" /usr/share/gr-kubernetes/scripts/start-kubelet.sh
        fi
        grep -q '^DNS_SERVERS' /etc/goodrain/envs/kubelet.sh || echo "DNS_SERVERS=${DNS%%|*}" >> /etc/goodrain/envs/kubelet.sh
        grep -q '^HOST_IP' /etc/goodrain/envs/kubelet.sh || echo "HOST_IP=$HOST_IP" >> /etc/goodrain/envs/kubelet.sh
    fi
    package::enable kubelet.service
    config_custom

    log.stdout '{
            "status":[ 
            { 
                "name":"install_kubelet", 
                "condition_type":"INSTALL_KUBELET", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

case $1 in
    *)
    run
    ;;
esac

