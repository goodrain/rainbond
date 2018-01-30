#!/bin/bash

function log.info() {
  echo "       $*"
}
 
function log.error() {
  echo " !     $*"
  echo ""
}

function log.stdout() {
    echo "$*" >&2
}

function prepare() {
    log.info "checking service dns..."
}
#循环解析
function is_normal() {
    log.info "checking service dns..."
    addrs="goodrain.me lang.goodrain.me maven.goodrain.me console.goodrain.me \
    region.goodrain.me kubeapi.goodrain.me"
    log.info "dns is parsing"
    for addr in $addrs
    do
        ip1=$(timeout 5 dig $addr | grep ^$addr | awk '{print $5}')
        ip2=172.30.42.1
        if [ $ip1 != $ip2 ];then
            log.info "dns Parse $addr failure"
            return 1
        fi
    done
    ip3_md5=$(grep ^nameserver /etc/resolv.conf | awk '{print $2}' | sort -b | md5sum | awk '{print $1}')
    ip4_md5=$(grctl node list | grep manage | awk '{print $4}' | sort -b | md5sum | awk '{print $2}')
    if [ $ip3 = $ip4 ];then
        log.info "dns is healthy"
        return 0
    else
        log.info "dns is unhealthy"
        return 1
    fi
}

function run() {
    log.info "service dns check"
                is_normal && ( 
                    log.stdout '{
                        "status":[
                        {
                            "name":"check-service-dns",
                            "condition_type":"SERVICE_DNS_NORMAL",
                            "condition_status":"True"
                        }
                        ],
                        "exec_status":"Success",
                        "type":"check"
                        }' ) || ( 
                    log.stdout '{
                        "status":[
                        {
                            "name":"check-dns-normal",
                            "condition_type":"DNS_IS_ABNORMAL", 
                            "condition_status":"False" 
                        }
                        ],
                        "exec_status":"Success", 
                        "type":"check"
                        }' ) 
}

case $1 in
    * )
        prepare
        run
    ;;
esac