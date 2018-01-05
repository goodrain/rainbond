#!/bin/bash 

OS_VER=$1
DNS=$2


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
    if [[ $OS_VER =~ "7" ]];then
        systemctl restart $proc
    else
        stop $proc
    fi
    return 0
}

function proc::start(){
    proc=$1
    if [[ $OS_VER =~ "7" ]];then
        systemctl start $proc
    else
        start $proc
    fi
    return 0
}

function proc::restart(){
    proc=$1
    if [ "$OS_VER" == "ubuntu/trusty" ];then
        restart $proc
    else
        systemctl restart $proc
    fi
    return 0
}

function compose::config_update() {
    YAML_FILE=/etc/goodrain/docker-compose.yaml
    mkdir -pv `dirname $YAML_FILE`
    if [ ! -f "$YAML_FILE" ];then
        echo "version: '2.1'" > $YAML_FILE
    fi
    dc-yaml -f $YAML_FILE -u -
}

function rewrite_dns() {
    log.info "update rbd-dns docker-compose.yaml"
    old_dns=$(egrep '^nameserver' /etc/resolv.conf | head -5 | awk '{print $2}' | sort -u | xargs | tr ' ' ',')
    log.info "old dns config:$old_dns"
    RBD_IMAGE_DNS_NAME=$(jq --raw-output '."rbd-dns".name' /etc/goodrain/envs/rbd.json)
    RBD_IMAGE_DNS_VERSION=$(jq --raw-output '."rbd-dns".version' /etc/goodrain/envs/rbd.json)
    RBD_DNS=$RBD_IMAGE_DNS_NAME:$RBD_IMAGE_DNS_VERSION

    compose::config_update << EOF
services:
  rbd-dns:
    image: $RBD_DNS
    container_name: rbd-dns
    environment:
      - KUBEURL=http://127.0.0.1:8181
      - FORWARD=$old_dns,114.114.114.114
      - SKYDNS_DOMAIN=goodrain.me
      - RECORD_1=goodrain.me:172.30.42.1
      - RECORD_2=lang.goodrain.me:172.30.42.1
      - RECORD_3=maven.goodrain.me:172.30.42.1
      - RECORD_4=config.goodrain.me:172.30.42.1
      - RECORD_5=console.goodrain.me:172.30.42.1
      - RECORD_6=region.goodrain.me:172.30.42.1
      - RECORD_7=kubeapi.goodrain.me:172.30.42.1
      - RECORD_8=download.goodrain.me:172.30.42.1
    logging:
      driver: json-file
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF
    log.info "up rbd-dns"
    dc-compose up -d rbd-dns
    echo "$old_dns" > /etc/goodrain/envs/.dns
}

function check_config() {
    dest_md5=$(echo $DNS | tr ',' '\n' | sort -u | xargs | md5sum | awk '{print $1}')
    old_md5=$(egrep '^nameserver' /etc/resolv.conf | head -5 | awk '{print $2}' | sort -u | xargs | md5sum | awk '{print $1}')

    log.info "new dns md5sum: <$dest_md5>"
    log.info "old dns md5sum: <$old_md5>"

    if [ ! -f "/etc/goodrain/envs/.dns" ];then
        rewrite_dns
    fi
    if [ "$dest_md5" == "$old_md5" ];then
        log.info "check resolv.conf ok"
        return 0
    else
        log.info "check resolv.conf failed, need reconf"
        return 1
    fi

}

function write_resolv_confd() {
    log.info "write resolv_confd"
    for file in /etc/resolvconf/resolv.conf.d/*
    do
        sed -i -e 's/^[^#]/#&/' $file
    done

    rm -f /run/resolvconf/interface/*

    cat /dev/null > /etc/resolvconf/resolv.conf.d/head
    for nameserver in $(echo $DNS | tr ',' ' ' | sort -u)
    do
        echo nameserver $nameserver >> /etc/resolvconf/resolv.conf.d/head
    done
    resolvconf -u
}

function write_resolv() {
    log.info "write resolv"
    sed -i -e 's/^[^#]/#&/' /etc/resolv.conf
    for nameserver in $(echo $DNS | tr ',' ' ' | sort -u)
    do
        echo nameserver $nameserver >> /etc/resolv.conf
    done
}

function run() {
    log.info "setting resolv.conf"
    check_config || (
        log.info "update dns"
        if [ -L "/etc/resolv.conf" ];then
            write_resolv_confd
        else
            write_resolv
        fi


        # manage centos
        #proc::is_running docker && (
        #    proc::stop docker
        #    proc::start docker
        if [[ $OS_VER =~ 7 ]];then
            grep "manage" /etc/goodrain/envs/.role >/dev/null 2>&1
            if [ $? -eq 0 ];then
                #proc::stop docker
                #proc::start docker
                systemctl restart docker
                sleep 15
                log.info "manage role node need: restart docker"
            fi
        fi
    )

    log.stdout '{ 
            "status":[ 
            { 
                "name":"update_dns", 
                "condition_type":"UPDATE_DNS", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

case $1 in
    * )
        run
        ;;
esac