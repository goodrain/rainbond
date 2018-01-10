#!/bin/bash

OS_VERSION=$1
HOSTIP=${2:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}
ETCD_ENDPOINTS=${3:-$(cat /etc/goodrain/envs/etcd-proxy.sh | awk -F '=' '{print $2}')} #calico ETCD_ENDPOINTS


calico_node_image="hub.goodrain.com/dc-deploy/calico-node:v2.4.1"

function log.info() {
  echo "       $*"
}

function log.error() {
  echo " !!!     $*"
  echo ""
}

function log.stdout() {
    echo "$*" >&2
}

function prepare() {
    echo "prepare network env"
    [ -d "/etc/goodrain/envs" ] || mkdir -pv /etc/goodrain/envs
    docker pull $calico_node_image 
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
    if [ "$OS_VERSION" == "ubuntu/trusty" ];then
        restart $proc
    else
        systemctl restart $proc
    fi
    return 0
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
    if [[ "$OS_VERSION" =~ "7" ]];then
        yum install -y $pkgname
    else
        DEBIAN_FRONTEND=noninteractive apt-get update
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
            log.stdout '{
            "status":[ 
            { 
                "name":"start_network_calico-node_client", 
                "condition_type":"START_NETWORK_CALICO_NODE_CLIENT", 
                "condition_status":"False"
            } 
            ], 
            "exec_status":"Failure",
            "type":"install"
            }'
            exit $_EXIT
        fi
    else
        UUIT_U=$(echo "$UNIT" | awk -F '-' '{print $1}') #calico/midolman
        proc::is_running $UUIT_U || proc::start $UUIT_U
    fi
}


# calico-node mode

function check_env_config_calico() {
    grep "$HOSTIP" /etc/goodrain/envs/calico.sh >/dev/null 2>&1
    if [ $? -eq 0 ];then
        echo "calico config checked"
        return 0
    else
        echo "calico config check failed"
        return 1
    fi
    grep ""
}

function write_env_config_calico() {
    cat <<EOF > /etc/goodrain/envs/calico.sh
DEFAULT_IPV4=$HOSTIP
ETCD_ENDPOINTS=http://$ETCD_ENDPOINTS
NODE_IMAGE=$calico_node_image
EOF
    source /etc/goodrain/envs/calico.sh
    if [ "$OS_VERSION" == "ubuntu/trusty" ];then
        proc::is_running calico && proc::stop calico
        proc::start calico
    fi
}

function update_cni_calico() {
    if [ -f "/etc/goodrain/cni/net.d/10-calico.conf" ];then
        sed -i "s#127.0.0.1:2379#$ETCD_ENDPOINTS#g" /etc/goodrain/cni/net.d/10-calico.conf
        grep "$ETCD_ENDPOINTS" /etc/goodrain/cni/net.d/10-calico.conf >/dev/null
        if [ $? -eq 0 ];then
            return 0
        else
            return 1
        fi
    fi
}

function write_cni_calico() {
     [ -f "/etc/goodrain/cni/net.d/10-calico.conf" ] && mv /etc/goodrain/cni/net.d/10-calico.conf /etc/goodrain/cni/net.d/10-calico.conf.bak
     cat > /etc/goodrain/cni/net.d/10-calico.conf <<EOF
{
    "name": "calico-k8s-network",
    "cniVersion": "0.1.0",
    "type": "calico",
    "etcd_endpoints": "http://$ETCD_ENDPOINTS",
    "log_level": "info",
    "ipam": {
        "type": "calico-ipam"
    },
    "kubernetes": {
        "kubeconfig": "/etc/goodrain/kubernetes/admin.kubeconfig"
    }
}
EOF
}

function run_calico_node() {
    package::is_installed gr-calico || package::install gr-calico
    update_cni_calico || write_cni_calico
    check_env_config_calico || write_env_config_calico
    package::enable calico-node.service
    calico_num=$(docker ps | grep 'calico' | wc -l)
    sleep 30
    if [ $calico_num -eq 1 ];then
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_network_calico-node", 
                    "condition_type":"INSTALL_NETWORK_CALICO_NODE", 
                    "condition_status":"True"
                } 
                ], 
                "exec_status":"Success",
                "type":"install"
                }'
    else
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_network_calico-node", 
                    "condition_type":"INSTALL_NETWORK_CALICO_NODE", 
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Failure",
                "type":"install"
                }'
    fi
}

case $1 in
    * )
        prepare
        run_calico_node
        ;;
esac