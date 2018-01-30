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
    log.info "checking calico-node service..."
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

function is_normal() {
    log.info "checking calico-node's health..."
        container_name=$(docker ps | grep -i calico | awk '{print $1}')
        if [ -n $container_name ];then
            container_state=$(curl --unix-socket /var/run/docker.sock http://localhost/containers/json?$container_name \
            | python -m json.tool | grep -i state)
            if [[ $container_state =~ 'running' ]];then 
                service_status=$(calicoctl node status | head -1)
                if [[ $service_status =~ 'running' ]];then
                log.info "service calico-node is healthy"
                return 0
                fi
            else
                log.error "service calico-node is unhealthy"
            return 1
            fi
        else
            log.info "no such colico container"
        fi
}

function run() {
    log.info "service calico-node check"
    is_install gr-calico && ( 
        is_enable calico-node && ( 
            is_active calico-node && (
                is_normal && ( 
                    log.stdout '{
                        "status":[
                        {
                            "name":"check-service-calico-node",
                            "condition_type":"SERVICE_CALICO-NODE_NORMAL",
                            "condition_status":"True"
                        }
                        ],
                        "exec_status":"Success",
                        "type":"check"
                        }' ) || ( 
                    log.stdout '{
                        "status":[
                        {
                            "name":"check-calico-node-normal",
                            "condition_type":"CALICO-NODE_IS_ABNORMAL", 
                            "condition_status":"False" 
                        }
                        ],
                        "exec_status":"Success", 
                        "type":"check"
                        }' )) || ( 
                log.stdout '{
                    "status":[
                    {
                        "name":"check-calico-node-active",
                        "condition_type":"CALICO-NODE_ISNOT_ACTIVE", 
                        "condition_status":"False" 
                    }
                    ],
                    "exec_status":"Success", 
                    "type":"check"
                    }' ))) || ( 
        log.stdout '{
            "status":[
            {
                "name":"check-calico-node-installed",
                "condition_type":"CALICO-NODE_ISNOT_INSTALL", 
                "condition_status":"False" 
            }
            ],
            "exec_status":"Success", 
            "type":"check"
            }'
    )
}

case $1 in
    * )
        prepare
        run
    ;;
esac