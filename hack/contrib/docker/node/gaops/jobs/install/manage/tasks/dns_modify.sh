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

function check_config() {
    dest_md5=$(echo $DNS | tr ',' '\n' | sort -u | xargs | md5sum | awk '{print $1}')
    old_md5=$(egrep '^nameserver' /etc/resolv.conf | head -5 | awk '{print $2}' | sort -u | xargs | md5sum | awk '{print $1}')

    log.info "new dns md5sum: <$dest_md5>"
    log.info "old dns md5sum: <$old_md5>"
    if [ "$dest_md5" == "$old_md5" ];then
        log.info "check resolv.conf ok"
        return 0
    else
        log.info "check resolv.conf failed, need reconf"
        return 1
    fi

}

function write_resolv_confd() {
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