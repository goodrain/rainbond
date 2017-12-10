#!/bin/bash

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

check_manage=(storage k8s network)
taskid=()

function check_manage() {

    if [ "$1" = "storage" ];then
            df -h | grep "/grdata" 2>&1 >/dev/null
            if [ "$?" -ne 0 ];then
                showmount -e 127.0.0.1 | grep "grdata"  >/dev/null 2>&1
                if [ $? -ne 0 ];then 
                    taskid+=("install_storage")
                fi
            fi
    elif [ "$1" = "k8s" ];then
        if [ ! -d "/usr/share/gr-kubernetes/" ];then
            taskid+=("install_k8s")
        fi
    elif [ "$1" = "network" ];then
        if [ ! -d "/etc/goodrain/cni/net.d/" ];then
            taskid+=("install_network")
        fi
    else
        log.info ""
    fi

}

function run(){

    for plugin in ${check_manage[@]};do
            check_manage $plugin
    done
    task=($taskid)
    task_num=${#task[@]}
    if [ $task_num -eq 0 ];then
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_manage_ready", 
                    "condition_type":"INSTALL_MANAGE_READY", 
                    "condition_status":"True"
                } 
                ], 
                "exec_status":"Success",
                "type":"install"
                }'
    else
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_manage_ready", 
                    "condition_type":"INSTALL_MANAGE_READY",  
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Failure",
                "type":"install"
                }'
    fi
}

case $1 in
    *)
    run
    ;;
esac