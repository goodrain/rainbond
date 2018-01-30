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
    log.info "checking k8s service..."
}

function gr-kube-apiserver_check() {
    log.info "service kube-apiserver check"
    is_install gr-kube-apiserver && (
        is_enable kube-apiserver && ( 
            is_active kube-apiserver || ( 
                log.stdout '{
                "status":[
                {
                    "name":"check-kube-apiserver-active",
                    "condition_type":"KUBE-APISERVER_ISNOT_ACTIVE", 
                    "condition_status":"False" 
                }
                ],
                "exec_status":"Success", 
                "type":"check"
                }' ))) || ( 
        log.stdout '{
        "status":[
        {
            "name":"check-kube-apiserver-installed",
            "condition_type":"KUBE-APISERVER_ISNOT_INSTALL", 
            "condition_status":"False" 
        }
        ],
        "exec_status":"Success", 
        "type":"check"
        }'
    )
}

function gr-kube-controller-manager_check() {
    log.info "service kube-controller-manager check"
    is_install gr-kube-controller-manager && ( 
        is_enable kube-controller-manager && ( 
            is_active kube-controller-manager  || ( 
                log.stdout '{
                "status":[
                {
                    "name":"check-kube-controller-manager-active",
                    "condition_type":"KUBE-CONTROLLER-MANAGER_ISNOT_ACTIVE", 
                    "condition_status":"False" 
                }
                ],
                "exec_status":"Success", 
                "type":"check"
                }' ))) || ( 
        log.stdout '{
        "status":[
        {
            "name":"check-kube-controller-manager-installed",
            "condition_type":"KUBE-CONTROLLER-MANAGER_ISNOT_INSTALL", 
            "condition_status":"False" 
        }
        ],
        "exec_status":"Success", 
        "type":"check"
        }'
    )
}

function gr-kube-scheduler_check() {
    log.info "service kube-scheduler check"
    is_install gr-kube-scheduler && ( 
        is_enable kube-scheduler && ( 
            is_active kube-scheduler || ( 
                log.stdout '{
                "status":[
                {
                    "name":"check-kube-scheduler-active",
                    "condition_type":"KUBE-SCHEDULER_ISNOT_ACTIVE", 
                    "condition_status":"False" 
                }
                ],
                "exec_status":"Success", 
                "type":"check"
                }' ))) || ( 
        log.stdout '{
        "status":[
        {
            "name":"check-kube-scheduler-installed",
            "condition_type":"KUBE-SCHEDULER_ISNOT_INSTALL", 
            "condition_status":"False" 
        }
        ],
        "exec_status":"Success", 
        "type":"check"
        }'
    )
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
    log.info "checking k8s's health..."
    k8s_api=$(kubectl config view | grep server | awk '{print $2}')
    api_check=$(curl $k8s_api/healthz)
        if [[ $api_check =~ 'ok' ]];then
        log.info "service k8s is healthy"
        return 0
        else
        log.error "service k8s is unhealthy"
        return 1
        fi
}

function run() {
    gr-kube-apiserver_check
    gr-kube-controller-manager_check
    gr-kube-scheduler_check
    is_normal && ( 
        log.stdout '{
            "status":[
            {
                "name":"check-service-kube-scheduler",
                "condition_type":"SERVICE_KUBE_SCHEDULER_NORMAL",
                "condition_status":"True"
            }
            ],
            "exec_status":"Success",
            "type":"check"
            }' ) || ( 
        log.stdout '{
            "status":[
            {
                "name":"check-kube-scheduler-normal",
                "condition_type":"KUBE_SCHEDULER_IS_ABNORMAL", 
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