#!/bin/bash

OS_VERSION=${1:-centos7}


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

function package::enable() {
    UNIT=$1
    log.info "enable $UNIT"
    if [[ $OS_VERSION =~ "7" ]];then
        systemctl is-enabled $UNIT || systemctl enable $UNIT
    else
        UUIT_U=$(echo "$UNIT" | awk -F '.' '{print $1}') 
        proc::is_running $UUIT_U 
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

function proc::status() {
    log.info "check status $1"
    proc=$1
    proc_num=$(ps -ef | grep "etcd " | grep -v "grep" | wc -l)
    if [ $proc_num -gt 0 ];then
        if [[ $OS_VERSION =~ "7" ]];then
            systemctl status $proc | grep "Active" | grep -i "Runn"
            if [ $? -eq 0 ];then
                log.info "$proc is running"
                return 0
            else
                proc_info=$(systemctl status etcd | grep "Active" | awk '{print $2$3$4}')
                log.info "$proc is not running: <${proc_info%)*})>"
                return 1
            fi
        else
            proc::is_running $proc && (
                #log.info "$proc is running"
                return 0
            ) || (
                log.info "$proc is not running"
                return 1
            )
        fi
    else
        log.info "not found etcd proc"
        return 1
    fi
}

function prepare() {
    log.info "RBD: check basic service: etcd"
}


function install_etcd_check() {
    log.info "Install etcd check"
    package::is_installed gr-etcd && (
        log.info "etcd installed..."
        package::enable etcd
        log.info "check etcd Successful."
    ) || (
            log.error "etcd not install."
            log.stdout '{
                "status":[ 
                { 
                    "name":"check_install_etcd_faild", 
                    "condition_type":"CHECK_INSTALL_ETCD_FAILD", 
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Success",
                "type":"check"
                }'
            exit 1
    )
}

function run_etcd_check() {
    log.info "Run etcd check"
    proc::status etcd && (
        log.info "etcd status running"
    ) || (
        log.error "etcd not running"
        proc_info=$(systemctl status etcd | grep "Active" | awk '{print $2$3$4}')
        etcd_not_reason=$(echo  ${proc_info%)*} | tr "(" "_")
        log.stdout '{
                "status":[ 
                { 
                    "name":"check_run_etcd_faild_'"$etcd_not_reason"'", 
                    "condition_type":"CHECK_RUN_ETCD_FAILD_'"$(echo  ${proc_info%)*} | tr '(' '_' | tr 'a-z' 'A-Z')"'", 
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Success",
                "type":"check"
                }'
            exit 1
    )
}

function health_check() {
    log.info "health check etcd"
    log.info "check v2"
    curl -L http://127.0.0.1:2379/health | grep "true"
    if [ $? -eq 0 ];then
        log.info "check cluster-health"
        etcdctl cluster-health | grep "cluster is healthy"
        if [ $? -eq 0 ];then
            return 0
        else
            log.stdout '{
                        "status":[ 
                        { 
                            "name":"health_check_etcd_failed", 
                            "condition_type":"health_check_etcd_failed", 
                            "condition_status":"False"
                        } 
                        ],
                        "exec_status":"Success",
                        "type":"check"
                        }'
            return 1
        fi
    else
        log.stdout '{
                        "status":[ 
                        { 
                            "name":"health_check_etcd_failed", 
                            "condition_type":"health_check_etcd_failed", 
                            "condition_status":"False"
                        } 
                        ],
                        "exec_status":"Success",
                        "type":"check"
                        }'
        return 1
    fi
}

function run(){
    log.info "check etcd"
    install_etcd_check && run_etcd_check && health_check
    if [ $? -eq 0 ];then
            log.stdout '{
                        "status":[ 
                        { 
                            "name":"etcd_is_healthy", 
                            "condition_type":"ETCD_IS_HEALTHY", 
                            "condition_status":"True"
                        } 
                        ],
                        "exec_status":"Success",
                        "type":"check"
                        }'
    else
        log.info ""
    fi
}

case $1 in
    * )
        prepare
        run
    ;;
esac