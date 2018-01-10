#!/bin/bash

# This script will detect basic services
#
# The output should be like
#
# KEY1 VALUE1

#set -o errexit
set -o pipefail

REPO_VER=${2:-3.4.1}

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

# define basic services
check_basic_services=(docker storage db base_plugins acp_plugins)
check_manage_services=(network k8s plugins analysis check_manage)
check_compute_services=(storage_client docker_compute network_compute kubelet plugins_compute check_compute)

RELEASE_INFO=$(cat /etc/os-release | grep "^VERSION=" | awk -F '="' '{print $2}' | awk '{print $1}' | cut -b 1-5)
if [[ $RELEASE_INFO == "7" ]];then
    OS_VER='centos/7'
elif [[ $RELEASE_INFO =~ "14" ]];then
    OS_VER='ubuntu/trusty'
elif [[ $RELEASE_INFO =~ "16" ]];then
    OS_VER='ubuntu/xenial'
elif [[ $RELEASE_INFO =~ "9" ]];then
    OS_VER="debian/stretch"
else
    log.stdout "Release $(cat /etc/os-release | grep "PRETTY" | awk -F '"' '{print $2}') Not supported"
    exit 1
fi


existed=()
taskid=()
taskid_str=""

function prepare() {
    log.info "RBD: check services"
}

function check_basic() {
    log.info "check basic services: $1"
    if [ "$1" = "docker" ];then
        if [ ! -f "/etc/goodrain/envs/docker.sh" ];then
            taskid+=("install_docker")
        fi
    elif [ "$1" = "docker_compute" ];then
        if [ ! -f "/etc/goodrain/envs/docker.sh" ];then
            taskid+=("update_dns_compute")
            taskid+=("install_docker_compute")
        fi
    elif [ "$1" = "base_plugins" ];then
        plugins_num=$(dc-compose ps  | grep "repo" | wc -l)
        if [ $plugins_num -eq 0 ];then
            taskid+=("install_base_plugins")
        fi
    elif [ "$1" = "acp_plugins" ];then
        plugins_num=$(dc-compose ps | grep "app" | wc -l)
        if [ $plugins_num -eq 0 ];then
            taskid+=("install_acp_plugins")
            taskid+=("update_dns")
        fi
    elif [ "$1" = "db" ];then
        plugins_num=$(dc-compose ps | grep "db" | wc -l)
        if [ $plugins_num -lt 1 ];then
            taskid+=("install_db")
        fi
    elif [ "$1" = "storage" ];then
        df -h | grep "/grdata" >/dev/null
        if [ "$?" -ne 0 ];then
            showmount -e 127.0.0.1 | grep "grdata"  >/dev/null 2>&1
            if [ $? -ne 0 ];then 
                taskid+=("install_storage")
            fi
        fi
    elif [ "$1" = "storage_client" ];then
        df -h | grep "/grdata"  >/dev/null
        if [ "$?" -ne 0 ];then
            showmount -e 127.0.0.1 | grep "grdata"  >/dev/null 2>&1
            if [ $? -ne 0 ];then
                taskid+=("install_storage_client")
            fi
        fi
    elif [ "$1" = "k8s" ];then
        if [ ! -f "/usr/share/gr-kubernetes/scripts/start-kube-apiserver.sh" ];then
            taskid+=("install_k8s")
            taskid+=("install_webcli")
        fi
    elif [ "$1" = "kubelet" ];then
        if [ ! -f "/usr/share/gr-kubernetes/scripts/start-kubelet.sh" ];then
            taskid+=("install_kubelet")
        fi
    elif [ "$1" = "plugins" ];then
        plugins_num=$(dc-compose ps | grep "proxy" | wc -l)
        if [ $plugins_num -eq 0 ];then
            taskid+=("install_plugins")
            taskid+=("do_rbd_images")
        fi
    elif [ "$1" = "plugins_compute" ];then
        plugins_num=$(dc-compose ps | grep "proxy" | wc -l)
        if [ $plugins_num -eq 0 ];then
            taskid+=("install_plugins_compute")
        fi
    elif [ "$1" = "network" ];then
        if [ ! -d "/etc/goodrain/cni/net.d/" ];then
            taskid+=("install_network")
        fi
    elif [ "$1" = "network_compute" ];then
        if [ ! -d "/etc/goodrain/cni/net.d/" ];then
            taskid+=("install_network_compute")
        fi
    elif [ "$1" = "check_manage" ];then
        plugins_num=$(dc-compose ps | grep "proxy" | wc -l)
        if [ $plugins_num -eq 0 ];then
            taskid+=("install_manage_ready")
        fi
    elif [ "$1" = "check_compute" ];then
        plugins_num=$(dc-compose ps | grep "proxy" | wc -l)
        if [ $plugins_num -eq 0 ];then
            taskid+=("install_compute_ready")
        fi
    elif [ "$1" = "dns" ];then
            taskid+=("update_dns")
    else
        log.info ""
    fi
}

function run_basic() {
    for plugin in ${check_basic_services[@]};do
        log.info "$plugin"
        check_basic $plugin
    done

    log.stdout '{
        "global":{"OS_VER":"'$OS_VER'","REPO_VER":"'$REPO_VER'"},
        "status":[ 
        { 
            "name":"check_manage_base_services", 
            "condition_type":"CHECK_MANAGE_BASE_SERVICES", 
            "condition_status":"'$([ ${#taskid[@]} -ne 0 ] && echo "False" || echo "True")'", 
            "next_tasks":['$(
                for i in ${taskid[@]}
                do 
                    if [ $i = ${taskid[${#taskid[*]}-1]} ];then 
                        echo "\"$i\""
                    else 
                        echo "\"$i\","
                    fi
                done
            )'] 
        } 
        ],
        "exec_status":"Success",
        "type":"check"
    }'
}

function run_manage() {
    for plugin in ${check_manage_services[@]};do
        log.info "$plugin"
        check_basic $plugin
    done

    log.stdout '{
        "status":[ 
        { 
            "name":"check_manage_services", 
            "condition_type":"CHECK_MANAGE_SERVICES", 
            "condition_status":"'$([ ${#taskid[@]} -ne 0 ] && echo "False" || echo "True")'", 
            "next_tasks":['$(
                for i in ${taskid[@]}
                do 
                    if [ $i = ${taskid[${#taskid[*]}-1]} ];then 
                        echo "\"$i\""
                    else 
                        echo "\"$i\","
                    fi
                done
            )'] 
        } 
        ],
        "exec_status":"Success",
        "type":"check"
    }'
}

function run_compute() {
    grep "manage" /etc/goodrain/envs/.role > /dev/null
    if [ $? -ne 0 ];then
        for plugin in ${check_compute_services[@]};do
            log.info "$plugin"
            check_basic $plugin
        done

        log.stdout '{
            "status":[ 
            { 
                "name":"check_compute_services", 
                "condition_type":"CHECK_COMPUTE_SERVICES", 
                "condition_status":"'$([ ${#taskid[@]} -ne 0 ] && echo "False" || echo "True")'", 
                "next_tasks":['$(
                    for i in ${taskid[@]}
                    do 
                        if [ $i = ${taskid[${#taskid[*]}-1]} ];then 
                            echo "\"$i\""
                        else 
                            echo "\"$i\","
                        fi
                    done
                )'] 
            } 
            ],
            "exec_status":"Success",
            "type":"check"
        }'
    else
        if [ ! -f "/usr/share/gr-kubernetes/scripts/start-kubelet.sh" ];then
            taskid+=("install_kubelet_manage")
            taskid+=("install_compute_ready_manage")
        fi

        log.stdout '{
            "status":[ 
            { 
                "name":"check_compute_services", 
                "condition_type":"CHECK_COMPUTE_SERVICES", 
                "condition_status":"'$([ ${#taskid[@]} -ne 0 ] && echo "False" || echo "True")'", 
                "next_tasks":['$(
                    for i in ${taskid[@]}
                    do 
                        if [ $i = ${taskid[${#taskid[*]}-1]} ];then 
                            echo "\"$i\""
                        else 
                            echo "\"$i\","
                        fi
                    done
                )'] 
            } 
            ],
            "exec_status":"Success",
            "type":"check"
        }'
    fi
}

case $1 in
    manage_base )
        prepare
        run_basic
    ;;
    manage )
        prepare
        run_manage
    ;;
    compute )
        prepare
        run_compute
    ;;
esac


