#!/bin/bash

# This script will initialize the system
#
# configure mirrors grub dns.

set -o errexit
set -o pipefail


# define 
# MIP node ip
# REPO_VER goodrain mirrors version ，default 3.4.1
# INSTALL_TYPE default online

HOST_UUID=$1
ETCD_NODE=$2
NODE_TYPE=${3:-manage}
MIP=$4
REPO_VER=${5:-3.4.1}
INSTALL_TYPE=${6:-online}
FIRST_NODE_TYPE=$3


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

function log.section() {
    local title=$1
    local title_length=${#title}
    local width=$(tput cols)
    local arrival_cols=$[$width-$title_length-2]
    local left=$[$arrival_cols/2]
    local right=$[$arrival_cols-$left]

    echo ""
    printf "=%.0s" `seq 1 $left`
    printf " $title "
    printf "=%.0s" `seq 1 $right`
    echo ""
}

# check os-release ,now support CentOS 7.x, Maybe support Ubuntu 16.04 & Debian 9
RELEASE_INFO=$(cat /etc/os-release | grep "^VERSION=" | awk -F '="' '{print $2}' | awk '{print $1}' | cut -b 1-5)
if [[ $RELEASE_INFO == "7" ]];then
    OS_VER='centos/7'
elif [[ $RELEASE_INFO =~ "14" ]];then
    OS_VER='ubuntu/trusty'
elif [[ $RELEASE_INFO =~ "16" ]];then
    OS_VER='ubuntu/xenial'
elif [[ $RELEASE_INFO =~ "9" ]];then
    OS_VER="debian/stretch"
else
    log.stdout "Release $(cat /etc/os-release | grep "PRETTY" | awk -F '"' '{print $2}') Not supported"
    exit 1
fi

# define install func
function package::match(){
    str=$1
    egrep_compile=$2
    echo $str | egrep "$egrep_compile" >/dev/null
}

function package::is_installed() {
    log.info "check package install"
    pkgname=$1
    if [[ $OS_VER =~ "7" ]];then
        rpm -qi $pkgname >/dev/null 2>&1
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

    if [[ $OS_VER =~ "7" ]];then
        yum install -y $pkgname >/dev/stdout 2>&1
    else
        DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes -o Dpkg::Options::="--force-confold"  $package="$pkg_version" >/dev/stdout 2>&1
    fi
}

function package::enable() {
    UNIT=$1
    if [[ $OS_VER =~ "7" ]];then
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
            "global":{"OS_VER":"'$OS_VER'","REPO_VER":"'$REPO_VER'"}, 
            "status":[ 
            { 
                "name":"start_service_'$UNIT'_failed", 
                "condition_type":"start_service_'$UNIT'_failed", 
                "condition_status":"False"
            } 
            ], 
            "type":"check"
            }'
            exit 1
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

function status::check() {
    UNIT=$1
    echo "check $UNIT on $OS_VER status"
    if [[ $OS_VER =~ '7' ]];then
        _EXIT=1
        for ((i=1;i<=3;i++ )); do
            sleep 1
            systemctl start $UNIT
            systemctl is-active $UNIT && export _EXIT=0 && break 
              
        done

        if [ $_EXIT -ne 0 ];then
            log.error "check $UNIT failed. abort..."
            log.stdout '{
            "global":{"OS_VER":"'$OS_VER'","REPO_VER":"'$REPO_VER'"}, 
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
    fi
}

function do_host_uuid() {
    if [ ! -z $HOST_UUID ];then
        echo "host_uuid=$HOST_UUID" > /etc/goodrain/host_uuid.conf
    fi
    log.info "host_uuid created"
}

function prepare() {
    log.section "ACP: Initialize the system"
    [ ! -d /etc/goodrain/envs ] && mkdir -p /etc/goodrain/envs || log.info ""
    log.info "prepare --> initialize the system"
    log.info "Install the system prerequisite package..."
    do_host_uuid
    if [[ $OS_VER =~ "7" ]];then
        yum makecache >/dev/stdout  2>&1
        yum install -y lsof htop rsync net-tools telnet iproute bind-utils tree >/dev/stdout 2>&1
    else
        apt update >/dev/stdout  2>&1
        apt install  -y lsof htop rsync net-tools telnet iproute lvm2 tree >/dev/stdout  2>&1
    fi
    
}

# check route、firewaall、dns
function check_system_services(){

    log.info "Check default gateway..."
    haveGW=`route -n| grep UG|awk '{print $2}' | head -1`
    if [ "$haveGW" != "" ];then
        log.info "Default Gateway: $haveGW"
    else
        log.error "Failure,not found default gateway.\nPlease set the default route.\nUse route add default gw route-ipaddress to set."
        exit 1
    fi

    # check localhost in /etc/hosts
    if [ ! "$(grep localhost /etc/hosts)" ];then
        echo -e "127.0.0.1\tlocalhost" >> /etc/hosts
    fi

    # init docker config
    [ ! -d ~/.docker ] && mkdir -pv ~/.docker
    [ ! -f ~/.docker/config.json ] && echo "{}" > ~/.docker/config.json

    # check unnecessary service
    log.info "Check unnecessary service..."
    if [[ $OS_VER =~ "7" ]];then
        log.info "disable firewalld"
        systemctl stop firewalld \
        && systemctl disable firewalld >/dev/stdout  2>&1

        log.info "disable NetworkManager"
        systemctl stop NetworkManager \
        && systemctl disable NetworkManager >/dev/stdout  2>&1

        log.info "Check dns..."
        
        systemctl stop dnsmasq >/dev/stdout  2>&1
        sed -i 's/^dns=dnsmasq/#&/' /etc/NetworkManager/NetworkManager.conf
    fi

    if [[ "$(lsof -i:53 | wc -l)" -ne 0 ]];then
        lsof -i:53 | grep -v 'PID' | awk '{print $2}' | uniq | xargs kill -9
        if [[ "$?" -eq 0 ]];then
            log.info "stop dnsmasq"
        fi
    fi
    if [[ "$(lsof -i:5353 | wc -l)" -ne 0 ]];then
        lsof -i:5353 | grep -v 'PID' | awk '{print $2}' | uniq | xargs kill -9
        if [[ "$?" -eq 0 ]];then
            log.info ""
        fi
    fi
}

# configure apt/yum mirrors
function config_mirrors(){
    if [ ! -n "$REPO_VER" ];then
        log.stdout '{
        "global":{"OS_VER":"'$OS_VER'"}, 
        "status":[ 
        { 
            "name":"init_config_mirrors", 
            "condition_type":"INIT_CONFIG_MIRRORS", 
            "condition_status":"False"
        } 
        ], 
        "type":"install"
        }'
        exit 1
    fi

    if [ "$INSTALL_TYPE" == "local" ];then
        if [ ! -f /etc/yum.repos.d/acp.repo ];then
            yum clean all >/dev/stdout  2>&1 \
            && rm -rf /etc/yum.repos.d/*

            cat >/etc/yum.repos.d/acp.repo <<EOF
[acp-local]
name=local
baseurl=file://$PWD/repo
enabled=1
gpgcheck=0
EOF

            yum makecache

            log.info "Install the system prerequisite package..."
            yum install -y perl telnet bind-utils htop dstat mariadb net-tools lsof iproute rsync lvm2 >/dev/stdout  2>&1
        fi
    else
        if [[ $OS_VER =~ '7' ]];then
            log.info "Configure yum repo..."
            cat >/etc/yum.repos.d/acp.repo <<EOF
[goodrain]
name=goodrain CentOS-\$releasever - for x86_64
baseurl=http://repo.goodrain.com/centos/\$releasever/${REPO_VER}/\$basearch
enabled=1
gpgcheck=1
gpgkey=http://repo.goodrain.com/gpg/RPM-GPG-KEY-CentOS-goodrain
EOF
            yum makecache >/dev/stdout  2>&1
        else
            log.info "Configure apt sources.list..."
            if [[ $OS_VER =~ '16' ]];then
                echo deb http://repo.goodrain.com/ubuntu/16.04 ${REPO_VER} main | tee /etc/apt/sources.list.d/acp.list 
            else
                echo deb http://repo.goodrain.com/ubuntu/14.04 ${REPO_VER} main | tee /etc/apt/sources.list.d/acp.list                 
            fi
            curl http://repo.goodrain.com/gpg/goodrain-C4CDA0B7 2>/dev/null | apt-key add - \
            && apt update >/dev/stdout  2>&1 \
            && apt install  -y lsof htop rsync net-tools telnet iproute lvm2 >/dev/stdout  2>&1
        fi
    fi
}

function config_grup(){
    configured=$(cat /etc/default/grub|grep "swapaccount=1")
    if [ "$configured" == "" ];then
      #echo "limit swap"
      echo GRUB_CMDLINE_LINUX="cgroup_enable=memory swapaccount=1" >> /etc/default/grub 
        if [[ $OS_VER =~ '7' ]];then
            grub2-mkconfig -o  /boot/grub2/grub.cfg >/dev/stdout  2>&1
        else
            grub-mkconfig -o /boot/grub/grub.cfg >/dev/stdout 2>&1
        fi
    fi
}

function config_ip(){
    mkdir -p /etc/goodrain/envs
    IP_INFO=$(ip ad | grep 'inet ' | egrep ' 10.|172.|192.168' | awk '{print $2}' | cut -d '/' -f 1 | grep -v '172.30.42.1')
    if [ -z $MIP ];then
        IP_ITEMS=($IP_INFO)
        MIP=${IP_ITEMS[0]}
    fi
    echo $IP_INFO | grep $MIP > /dev/null
    if [ $? -eq 0 ];then
        echo "LOCAL_IP=$MIP" > /etc/goodrain/envs/ip.sh
    else
        echo "MANAGE_IP=$MIP" > /etc/goodrain/envs/ip.sh
    fi 
}

function config_system() {

    log.info "configure system limit"
    file_max=$(sysctl fs.file-max | awk -F '[ =]' '{print $4}')
    if [[ $file_max -lt 100000 ]];then
        echo -e "\e[31m configure fs.lile-max = 100000 .\e[0m"
        grep "fs.file-max" /etc/sysctl.conf >/dev/null
        if [ $? -ne 0 ];then
            echo fs.file-max=100000 >> /etc/sysctl.conf
        else
            sed -i "s/fs.file-max.*/fs.file-max=100000/g" /etc/sysctl.conf
        fi
         sysctl -p | grep "fs.file-max"
    fi
    grep "root soft nofile" /etc/security/limits.conf >/dev/null
    if [ $? -eq 0 ];then
        limit_max=$(grep "root soft nofile" /etc/security/limits.conf | awk '{print $4}')
        if [ $limit_max -lt 60000 ];then
            sed -i "s/root soft nofile.*/root soft nofile 65535/g" /etc/security/limits.conf
            sed -i "s/root hard nofile.*/root hard nofile 65535/g" /etc/security/limits.conf
            sed -i "s/\* soft nofile.*/\* soft nofile 65535/g" /etc/security/limits.conf
            sed -i "s/\* soft nofile.*/\* soft nofile 65535/g" /etc/security/limits.conf
        fi
    else
        echo "
root soft nofile 65535
root hard nofile 65535
* soft nofile 65535
* hard nofile 6553" >> /etc/security/limits.conf 
    fi
}

function init_system() {
    log.info "Initialize the system"

    check_system_services && config_mirrors && config_grup && config_ip && config_system
    if [ $? -eq 0 ];then
        return 0
    else
        return 1
    fi
}

function write_env_config() {
    log.info "config etcd.sh"
    if [ -f "/etc/goodrain/envs/etcd.sh" ];then
        grep "$MIP" /etc/goodrain/envs/etcd.sh >/dev/null || (
            echo "LOCAL_IP=$MIP" > /etc/goodrain/envs/etcd.sh
        )
        log.info "config etcd.sh ok"
    else
        echo "LOCAL_IP=$MIP" >> /etc/goodrain/envs/etcd.sh
    fi
}

function rewrite_conf() {
    CONFIG=/usr/share/gr-etcd/scripts/start.sh
    cp $CONFIG $CONFIG.`date +%s`
    #ETCD_NODE aaaa-127.0.0.1,bbbb-127.0.0.2
    for node in $(echo $ETCD_NODE | tr " " "," | sort -u )
    do
        node_name=${node%%-*}
        node_ip=${node#*-}
        sed -i "/\$LOCAL_NODE:\$LOCAL_IP/a${node_name}:${node_ip}" /usr/share/gr-etcd/scripts/start.sh
    done
    # reg
    xxx=${ETCD_NODE%%,*}
    MASTER_NODE=${xxx#*-}
    curl http://${MASTER_NODE}:2379/v2/members -XPOST -H "Content-Type: application/json" -d '{"peerURLs": ["http://'${MIP}':2380"]}'
    #sed -i "s/\$LOCAL_NODE:\$LOCAL_IP/$ETCD_NODE/g" /usr/share/gr-etcd/scripts/start.sh
    #sed -i "/\$LOCAL_NODE:\$LOCAL_IP/a${NODE}"
    sed -i 's/INITIAL_CLUSTER_STATE=""/INITIAL_CLUSTER_STATE="existing"/g' /usr/share/gr-etcd/scripts/start.sh
}

function install_etcd() {
    log.info "install etcd"
    if [[ "$NODE_TYPE" == "manage"  ]];then
        package::is_installed gr-etcd  || (
                package::install gr-etcd  || (
                    log.error "install faild"
                    log.stdout '{
                    "global":{"OS_VER":"'$OS_VER'","REPO_VER":"'$REPO_VER'"}, 
                    "status":[ 
                    { 
                        "name":"install_etcd_manage", 
                        "condition_type":"INSTALL_ETCD_MANAGE", 
                        "condition_status":"False"
                    } 
                    ], 
                    "type":"install"
                    }'
                    exit 1
                )
            )
            package::is_installed gr-etcdctl  || (
                package::install gr-etcdctl  || (
                    log.error "install faild"
                    log.stdout '{
                    "global":{"OS_VER":"'$OS_VER'","REPO_VER":"'$REPO_VER'"}, 
                    "status":[ 
                    { 
                        "name":"install_etcdctl_manage", 
                        "condition_type":"INSTALL_ETCDCTL_MANAGE", 
                        "condition_status":"False"
                    } 
                    ], 
                    "type":"install"
                    }'
                    exit 1
                )
            )
            write_env_config
        if [ ! -z "$ETCD_NODE" ];then
            # etcd 扩容暂时不动
            rewrite_conf
        fi
        package::enable etcd || status::check etcd
        log.info "install etcd ok."
    else
        package::is_installed gr-etcd-proxy  || (
            package::install gr-etcd-proxy  || (
                log.error "install faild"
                return 1
            )
        )
        [ -f "/etc/goodrain/envs/etcd-proxy.sh" ] && rm /etc/goodrain/envs/etcd-proxy.sh
        echo "MASTER_IP=$ETCD_NODE:2379" > /etc/goodrain/envs/etcd-proxy.sh
        package::enable etcd-proxy || status::check etcd-proxy
    fi
}

function run(){

    log.info "role"
    if [ $NODE_TYPE == "manage" ];then
        if [ -z $FIRST_NODE_TYPE ];then
            echo "role:manage,compute" > /etc/goodrain/envs/.role
        else
            echo "role:manage" > /etc/goodrain/envs/.role
        fi
    else
        echo "role:compute" > /etc/goodrain/envs/.role
    fi

    init_system && ( log.info "" ) || (
        log.stdout '{
        "global":{"OS_VER":"'$OS_VER'","REPO_VER":"'$REPO_VER'"}, 
        "status":[ 
        { 
            "name":"init_system_rainbond", 
            "condition_type":"INIT_SYSTEM_RAINBOND", 
            "condition_status":"False"
        } 
        ], 
        "type":"install"
        }'
        exit 1
    )
    install_etcd
}


case $1 in
    * )
        prepare
        run
        ;;
esac
