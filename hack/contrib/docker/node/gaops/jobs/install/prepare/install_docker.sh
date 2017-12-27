#!/bin/bash

# This script will detect basic services
#
# The output should be like
#
# KEY1 VALUE1
#
# Basic services For ACP install:docker etcd/etcd(etcd_proxy)/acp-node

#set -o errexit
set -o pipefail

# define service name
# centos/7 docker 
# centos/7 etcd manage <ip>
# centos/7 etcd compute (etcd_proxy) <ip>
# centos/7 acp-node manage (master mode)
# centos/7 acp-node compute


OS_VERSION=$1
INSTALL_SERVICE=$2
NODE_TYPE=$3
HOST_IP=${4:-127.0.0.1}


[ -z $TERM ] && TERM=xterm-256color
#ETCD_NODE=$5

# define log func 
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

# define install func
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
        yum install -y $pkgname 2>&1
    else
        DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes -o Dpkg::Options::="--force-confold"  $package="$pkg_version" 2>&1
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
            systemctl is-active $UNIT && export _EXIT=0 && break
        done

        if [ $_EXIT -ne 0 ];then
            log.error "check failed. abort..."
            log.stdout '{ 
            "status":[ 
            { 
                "name":"start_service_'$UNIT'_failed", 
                "condition_type":"start_service_'$UNIT'_failed", 
                "condition_status":"False"
            } 
            ], 
            "type":"check"
            }'
            exit $_EXIT
        fi
    else
        UUIT_U=$(echo "$UNIT" | awk -F '.' '{print $1}') # docker
        proc::is_running $UUIT_U || proc::start $UUIT_U
    fi
}

# define status
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
    if [ "$RELEASE" == "ubuntu/trusty" ];then
        restart $proc
    else
        systemctl restart $proc
    fi
    return 0
}

function add_user() {
    grep rain /etc/group >/dev/null 2>&1 || groupadd -g 200 rain
    id rain >/dev/null 2>&1 || (
        useradd -m -s /bin/bash -u 200 -g 200 rain
        echo "rain ALL = (root) NOPASSWD:ALL" > /etc/sudoers.d/rain
        chmod 0440 /etc/sudoers.d/rain
    )
    log.info "add_user ok"
}

function prepare() {
    log.info "RBD: install basic service: docker"
    [ -d "/etc/goodrain/envs" ] || mkdir -p /etc/goodrain/envs
    [ -d "/root/.docker" ] || mkdir -p /root/.docker
    [ -f "/root/.docker/config.json" ] || echo "{}" >> /root/.docker/config.json
    log.info "prepare docker..."
    log.info "add rain user"
    add_user
}

function write_docker_config() {
    #
    # --dns-opt=use-vc centos7.4
    #
    log.info "write_docker_config"
    if [ "$OS_VERSION" == "ubuntu/trusty" ];then
            echo 'DOCKER_OPTS="-H 0.0.0.0:2376 -H unix:///var/run/docker.sock --bip=172.30.42.1/16 --insecure-registry goodrain.me"' > /etc/default/docker
    else
        [ -s "/etc/goodrain/envs/docker.sh" ] || (
            if [ -f /etc/lvm/profile/docker-thinpool.profile ];then
                cat > /etc/goodrain/envs/docker.sh << END
DOCKER_OPTS=" -H 0.0.0.0:2376 -H unix:///var/run/docker.sock \
--bip=172.30.42.1/16 \
--insecure-registry goodrain.me \
--insecure-registry hub.goodrain.com \
--storage-driver=devicemapper \
--storage-opt=dm.thinpooldev=/dev/mapper/docker-thinpool \
--storage-opt=dm.use_deferred_removal=true \
--storage-opt=dm.use_deferred_deletion=true \
--userland-proxy=false"
END
            else
                echo 'DOCKER_OPTS=" -H 0.0.0.0:2376 -H unix:///var/run/docker.sock --bip=172.30.42.1/16 --insecure-registry goodrain.me --userland-proxy=false"' > /etc/goodrain/envs/docker.sh
            fi
        )
    fi  
}

function docker_mirrors() {
    log.info "configure docker mirrors"
    [ ! -f "/etc/docker/daemon.json" ] && mkdir /etc/docker/ || mv /etc/docker/daemon.json /etc/docker/daemon.json.old
    cat >/etc/docker/daemon.json <<EOF
{
  "registry-mirrors": ["https://registry.docker-cn.com"]
}
EOF
    docker_num=$(ps -ef | grep docker | grep -v 'grep' | wc -l)
    if [ $docker_num -eq 0 ];then
        proc::restart docker
    else
        log.info "docker dont't need restart "
    fi
}

function install_docker() {
    log.info "Install docker"
    package::is_installed gr-docker-engine  || (
        package::install gr-docker-engine  || (
            log.error "install faild"
            log.stdout '{
                "status":[ 
                { 
                    "name":"install_docker_faild", 
                    "condition_type":"INSTALL_DOCKER_FAILD", 
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Failure",
                "type":"common"
                }'
            exit 1
        )
    )
    write_docker_config && docker_mirrors
    package::enable docker.service

    which dps > /dev/null 2>&1 || (
    docker run --rm -v /var/run/docker.sock:/var/run/docker.sock hub.goodrain.com/dc-deploy/archiver gr-docker-utils
    docker run --rm -v /var/run/docker.sock:/var/run/docker.sock hub.goodrain.com/dc-deploy/archiver gr-docker-compose
    ) 
    log.info "Install docker Successful."
}

function run(){
    log.info "install docker"
    install_docker
    docker ps >/dev/null
    if [ $? -eq 0 ];then
            log.stdout '{
                        "status":[ 
                        { 
                            "name":"install_docker", 
                            "condition_type":"INSTALL_DOCKER", 
                            "condition_status":"True"
                        } 
                        ],
                        "exec_status":"Success",
                        "type":"install"
                        }'
    fi
}

case $1 in
    * )
        prepare
        run
    ;;
esac