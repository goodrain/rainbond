#!/bin/bash
set -o errexit
set -o pipefail

OS_VERSION=$1
CALICO_NET=$2 #172.16.0.0/16
ETCD_ENDPOINTS=${3:-127.0.0.1:2379} #calico ETCD_ENDPOINTS:2379
HOSTIP=${4:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}


# 获取ip
calico_node_image="hub.goodrain.com/dc-deploy/calico-node:v2.4.1"


if [ -z $CALICO_NET ];then
    IP_INFO=$(ip ad | grep 'inet ' | egrep ' 10.|172.|192.168' | awk '{print $2}' | cut -d '/' -f 1 | grep -v '172.30.42.1')
    IP_ITEMS=($IP_INFO)
    INET_IP=${IP_ITEMS%%.*}
    if [ $INET_IP = '172' ];then
        CALICO_NET=10.0.0.0/16
    elif [ $INET_IP = '10' ];then
        CALICO_NET=172.16.0.0/16
    else
        CALICO_NET=172.16.0.0/16
    fi
fi

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
    log.info "install network plugins calico-node"
    log.info "prepare network env"
    [ -d "/etc/goodrain/envs" ] || mkdir -pv /etc/goodrain/envs
    docker pull $calico_node_image
}

function proc::is_running() {
    proc=$1
    proc_info=$(status $proc 2>&1)
    proc_items=($proc_info)
    status=${proc_items[1]%/*}
    if [ "$status" == "start" ];then
        log.info "$proc is running"
        return 0
    else
        log.info "$proc is not running: <$proc_info>"
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
    if [[ $OS_VERSION =~ "7" ]];then
        yum install -y $pkgname
    else
        DEBIAN_FRONTEND=noninteractive apt-get update
        DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes -o Dpkg::Options::="--force-confold"  $package="$pkg_version"
    fi

}

function package::enable() {
    UNIT=$1
    if [[ $OS_VERSION =~ "7" ]];then
        systemctl is-enabled $UNIT || systemctl enable $UNIT
        systemctl is-active $UNIT || systemctl start $UNIT
        _EXIT=1
        for ((i=1;i<=3;i++ )); do
            sleep 1
            systemctl is-active $UNIT || systemctl start $UNIT && export _EXIT=0 && break
        done

        if [ $_EXIT -ne 0 ];then
            log.error "check failed. abort..."
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
        log.info "calico config checked"
        return 0
    else
        log.info "calico config check failed"
        return 1
    fi
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

function check_calico_pool() {
    CALICO_NET_ETCD=$(echo $CALICO_NET | sed 's#/#-#g')
    log.info "CALICO_NET:$CALICO_NET to $CALICO_NET_ETCD"
    etcdctl get /calico/v1/ipam/v4/pool/$CALICO_NET_ETCD >/dev/null 2>&1
    if [ $? -eq 0 ];then
        log.info "calico pool config checked"
        return 0
    else
        log.info "calico pool config check failed"
        return 1
    fi
}

function reconfig_calico_pool () {
    source /etc/goodrain/envs/calico.sh
    for path in $(etcdctl ls /calico/v1/ipam/v4/pool)
    do
        etcdctl rm $path
    done

cat - <<EOF | calicoctl create -f -
apiVersion: v1
kind: ipPool
metadata:
  cidr: $CALICO_NET
spec:
  ipip:
    enabled: true
    mode: cross-subnet
  nat-outgoing: true
  disabled: false
EOF

}

function run_calico_node() {
    log.info "setup calico-manage"
    package::is_installed gr-calico $OS_VERSION || package::install gr-calico $OS_VERSION
    check_env_config_calico || write_env_config_calico
    check_calico_pool || reconfig_calico_pool
    package::enable calico-node.service $OS_VERSION

    log.stdout '{
            "global":{
              "CALICO_NET":"'$CALICO_NET'"
            },
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
}

case $1 in
    * )
        prepare
        run_calico_node
        ;;
esac