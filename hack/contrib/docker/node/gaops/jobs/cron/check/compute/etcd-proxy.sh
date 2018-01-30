#!/bin/bash

OS_VERSION=${1:-centos7}

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
    log.info "checking etcd-proxy service..."
}

#区分系统版本,centos与debian系列
function is_install() {
    pkgname=$1
    log.info "checkinging $pkgname install..."
    if [[ $OS_VERSION =~ "7" ]];then
        rpm -qi $pkgname > /dev/null
        if [ $? -eq 0 ];then
            log.info "service $pkgname is installed"
        return 0
        else
            log.error "service $pkgname is not install"
        return 1
        fi
    else
        apt search $pkgname | grep installed >/dev/null
        if [ $? -eq 0 ];then
            log.info "service $pkgname is installed"
        return 0
        else
            apt search $pkgname | grep upgradable >/dev/null
            if [ $? -eq 0 ];then
                log.info "service $pkgname is installed"
            return 0
            else
                log.error "service $pkgname is not install"
            return 1
            fi
        fi
    fi
}

function is_enable() {
    UNIT=$1
    log.info "checking $UNIT enabled..."
        check_enable=$(systemctl is-enabled $UNIT)
        if [ $check_enable = "enabled" ];then
            log.info "$UNIT is enable"
        return 0
        else
            log.error "$UNIT is not enable"
            systemctl enable $UNIT
        return 0
        fi
}

#循环三次定时检测，防止restart
function is_active() {
    UNIT=$1
    log.info "checking $UNIT active..."
    for (( i=1; i <= 3; i++ ))
    do
    sleep 3
    check_active=$(systemctl is-active $UNIT)
        if [ $check_active = "active" ];then 
            log.info "$UNIT is running"
        else
            log.error "$UNIT is not running"
        return 1
        fi
    done
    return 0
}
#测试可用:通过代理发送请求
function is_normal() {
    log.info "checking etcd-proxy's health..."
    ETCDCTL_API=3 etcdctl --endpoints=127.0.0.1:2379 member list | grep started > /dev/null
    if [ "$?" -eq 0 ];then
        log.info "etcd-proxy is normal"
        return 0
    else
        log.info "etcd-proxy is abnormal"
        return 1
    fi
}

function run() {
    node_type=$(cat /etc/goodrain/envs/.role | awk -F ':' '{print$2}')
    if [ $node_type = "compute" ];then
    log.info "service ectd-proxy check"
    is_install gr-etcd-proxy && ( 
        is_enable etcd-proxy && ( 
            is_active etcd-proxy && (
                is_normal && ( 
                    log.stdout '{
                        "status":[
                        {
                            "name":"check-service-ectd-proxy",
                            "condition_type":"SERVICE_ETCD-PROXY_NORMAL",
                            "condition_status":"True"
                        }
                        ],
                        "exec_status":"Success",
                        "type":"check"
                        }' ) || ( 
                    log.stdout '{
                        "status":[
                        {
                            "name":"check-ectd-proxy-normal",
                            "condition_type":"ETCD-PROXY_IS_ABNORMAL", 
                            "condition_status":"False" 
                        }
                        ],
                        "exec_status":"Success", 
                        "type":"check"
                        }' )) || ( 
                log.stdout '{
                    "status":[
                    {
                        "name":"check-ectd-proxy-active",
                        "condition_type":"ETCD-PROXY_ISNOT_ACTIVE", 
                        "condition_status":"False" 
                    }
                    ],
                    "exec_status":"Success", 
                    "type":"check"
                    }' ))) || ( 
        log.stdout '{
            "status":[
            {
                "name":"check-ectd-proxy-installed",
                "condition_type":"ETCD-PROXY_ISNOT_INSTALL", 
                "condition_status":"False" 
            }
            ],
            "exec_status":"Success", 
            "type":"check"
            }'
    )
    else
    log.info "this is manage node"
    log.stdout '{
        "status":[
        {
            "name":"check-service-ectd-proxy",
            "condition_type":"SERVICE_ETCD-PROXY_NORMAL",
            "condition_status":"True"
        }
        ],
        "exec_status":"Success",
        "type":"check"
        }'
    fi
}

case $1 in
    * )
        prepare
        run
    ;;
esac