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

check_compute=(storage kubelet network)
taskid=()

function check_compute() {

    if [ "$1" = "storage" ];then
            df -h | grep "/grdata" 2>&1 >/dev/null
            if [ "$?" -ne 0 ];then
                taskid+=("install_storage_client")
            fi
    elif [ "$1" = "kubelet" ];then
        if [ ! -f "/usr/share/gr-kubernetes/scripts/start-kubelet.sh" ];then
            taskid+=("install_kubelet")
        fi
    elif [ "$1" = "network" ];then
        if [ ! -d "/etc/goodrain/cni/net.d/" ];then
            taskid+=("install_network_compute")
        fi
    else
        log.info ""
    fi

}

function run_compute(){

    for plugin in ${check_compute[@]};do
            check_compute $plugin
    done
    task=($taskid)
    task_num=${#task[@]}
    if [ $task_num -eq 0 ];then
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_compute_ready", 
                    "condition_type":"INSTALL_COMPUTE_READY", 
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
                    "name":"install_compute_ready", 
                    "condition_type":"INSTALL_COMPUTE_READY",  
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Failure",
                "type":"install"
                }'
    fi
}

function run_manage() {
    if [ ! -f "/usr/share/gr-kubernetes/scripts/start-kubelet.sh" ];then
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_compute_ready", 
                    "condition_type":"INSTALL_COMPUTE_READY",  
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Failure",
                "type":"install"
                }'
    else
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_compute_ready", 
                    "condition_type":"INSTALL_COMPUTE_READY", 
                    "condition_status":"True"
                } 
                ], 
                "exec_status":"Success",
                "type":"install"
                }'
    fi
}


function run() {
    grep "manage" /etc/goodrain/envs/.role > /dev/null
    if [ $? -eq 0 ];then
        log.info "manage compute"
        run_manage
    else
        log.info "compute"
        run_compute
    fi
}
case $1 in
    *)
    run
    ;;
esac