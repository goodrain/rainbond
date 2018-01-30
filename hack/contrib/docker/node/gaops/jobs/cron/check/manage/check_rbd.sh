#!/bin/bash

#
# 自研组件监控
# api worker chaos mq entrance eventlog webcli
#

FW=$1

[ -z $FW ] && exit 1

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

function stdout::success(){
    log.stdout '{
            "status":[
            {
                "name":"check_status_health_'$FW'",
                "condition_type":"CHECK_STATUS_HEALTH_'$(echo $FW | tr [a-z] [A-Z])'", 
                "condition_status":"True" 
            }
            ],
            "exec_status":"Success", 
            "type":"check"
            }'
}

function stdout::failure(){
    INFO=$1
    log.stdout '{
            "status":[
            {
                "name":"'$INFO'_'$FW'",
                "condition_type":"'$(echo $INFO|tr [a-z] [A-Z])'_'$(echo $FW | tr [a-z] [A-Z])'", 
                "condition_status":"False" 
            }
            ],
            "exec_status":"Success", 
            "type":"check"
            }'
}

function prepare() {
    log.info "checking service $FW"
}

function container::check() {
    log.info "check $FW is_running?"
    dc-compose ps | grep "\<$FW.*Up\>" >/dev/null
    if [ $? -eq 0 ];then
        return 0
    else
        return 1
    fi
}

function api_check() {
    #log.info "check api service"
    
    container::check $FW && (
        log.info "check health $FW"
        http_code=$(curl -I -o /dev/null -s -w %{http_code} localhost:8888/v2/show)
        if [ $http_code == "200" ];then
            stdout::success 
        else
            stdout::failure "check_not_health"
        fi
    ) || (
        stdout::failure "check_not_running"
    )
    
}

function chaos_check() {
    
    container::check $FW && (
        # chaos api
        stdout::success 
    ) || (
        stdout::failure "check_not_running"
    )
}

function entrance_check() {
    
    container::check $FW && (
        http_code=$(curl -I -o /dev/null -s -w %{http_code} localhost:6200/metrics)
        if [ $http_code == "200" ];then
            stdout::success 
        else
            stdout::failure "check_not_health"
        fi
    ) || (
        stdout::failure "check_not_running"
    )
}

function mq_check() {
    
    container::check $FW && (
        # mq
        stdout::success
    ) || (
        stdout::failure "check_not_running"
    )
}

function worker_check() {
    
    container::check $FW && (
        stdout::success
    ) || (
        grep "worker" /etc/goodrain/docker-compose.yaml > /dev/null
        if [ $? -ne 0 ];then
            stdout::success 
        else
            stdout::failure "check_not_running"
        fi
    )
}

function eventlog_check() {
    
    container::check $FW && (
        stdout::success
    ) || (
        stdout::failure "check_not_running"
    )
}

function webcli_check(){
    
    container::check $FW && (

        stdout::success
    ) || (
        stdout::failure "check_not_running"
    )
}

function check_misc(){
    container::check $FW && (
        stdout::success
    ) || (
        grep "$FW$" /etc/goodrain/docker-compose.yaml > /dev/null
        if [ $? -ne 0 ];then
            stdout::success 
        else
            stdout::failure "check_not_running"
        fi
    )
}

function run() {
    log.info "start check"
    case $FW in
        api)
            api_check
        ;;
        chaos)
            chaos_check
        ;;
        worker)
            worker_check
        ;;
        mq)
            mq_check
        ;;
        entrance)
            entrance_check
        ;;
        eventlog)
            eventlog_check
        ;;
        webcli)
            webcli_check
        ;;
        *)
            check_misc
        ;;
    esac
    
}

case $1 in
    * )
        prepare
        run
    ;;
esac