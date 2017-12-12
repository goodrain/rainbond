#!/bin/bash

WEB=${1:-www.baidu.com}

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


function prepare() {
    log.section "check network demo"
    log.info "nothing for check network demo"
}

function curl() {
    log.info "network test:curl"
    rsp_code=$(curl -I www.baidu.com | grep "HTTP" | awk '{print $2}')
    if [[ $rsp_code -lt 400 ]];then
        return 0
    else
        return 1
    fi
}

function run() {
    curl $WEB && (
        log.stdout '{
        "status":[ 
        { 
            "name":"check_network_demo", 
            "condition_type":"CHECK_NETWORK_DEMO", 
            "condition_status":"True" 
        } 
        ],
        "exec_status":"Success",
        "type":"check"
    }'
    ) || (
        log.stdout '{
        "status":[ 
        { 
            "name":"check_network_demo", 
            "condition_type":"CHECK_NETWORK_DEMO_ERROR", 
            "condition_status":"False" 
        } 
        ],
        "type":"check"
    }'
    )
}

case $1 in
    *)
        prepare
        run
    ;;
esac