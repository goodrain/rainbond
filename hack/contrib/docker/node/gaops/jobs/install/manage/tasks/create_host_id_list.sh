#!/bin/bash 

REGION_API_IP=$1

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

[ -f "/tmp/host_id_list.conf" ] && rm -rf /tmp/host_id_list.conf

for api in $(echo $REGION_API_IP | tr ',' ' ' | sort -u)
do
    #log.info "$api"
    api_all=$(grctl node list | grep ${api%:*} | awk '{print $2"="$4}')
    api_node=${api_all##*-}
    #log.info "$api_node"
    echo "$api_node;" >> /tmp/host_id_list.conf
done

echo $ND_API_INFO

function check_config() {
    #old=""
    if [ -f "/etc/goodrain/host_id_list.conf" ];then
        old=$(cat /etc/goodrain/host_id_list.conf | awk -F ';' '{print $1}' | xargs | md5sum | awk '{print $1}')
    else
        log.info "host_id_list.conf not exist"
        old=$(echo "0" | md5sum | awk '{print $1}')
    fi

    log.info "old host_id_list.conf md5 <$old>"
    if [ -f "/tmp/host_id_list.conf" ];then
        new=$(cat /tmp/host_id_list.conf | awk -F ';' '{print $1}' | xargs | md5sum | awk '{print $1}')
    else
        log.info "host_id_list.conf cache not exist"
        new=$(echo "0" | md5sum | awk '{print $1}')
    fi
    log.info "wanted add host_id_list.conf md5 <$new>"
    if [ $new = $old ];then
        log.info "not change."
        return 0
    else
        log.info "need change.will update host_id_list.conf."
        return 1
    fi
}

function write_host_id_list() {
    [ -f "/etc/goodrain/host_id_list.conf" ] && mv /etc/goodrain/host_id_list.conf /etc/goodrain/host_id_list.conf.bak
    for api in $(echo $REGION_API_IP | tr ',' ' ' | sort -u)
    do
        api_all=$(grctl node list | grep ${api%:*} | awk '{print $2"="$4}')
        #log.info "$api,$api_all"
        api_node=${api_all##*-}

        log.info "add $api_node to host_id_list.conf"
        echo "$api_node;" >> /etc/goodrain/host_id_list.conf
    done
    log.info "will restart rbd-api"
    dc-compose restart rbd-api
    [ $? -ne 0 ] && (
        dc-compose stop rbd-api
        cclear
        dc-compose up -d
    )
    #dc-compose ps | grep "rbd-api" | grep -i "up"
}

function run() {
    log.info "create host_id_list for api"
    check_config || (
        write_host_id_list
    )

    log.stdout '{ 
            "status":[ 
            { 
                "name":"create_host_id_list", 
                "condition_type":"create_host_id_list", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

case $1 in
    * )
        run
        ;;
esac