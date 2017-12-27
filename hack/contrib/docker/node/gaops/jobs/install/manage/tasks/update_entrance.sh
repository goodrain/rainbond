#!/bin/bash 

ENTRANCE_IP=$1

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

RBD_ENTRANCE_VER=$(jq --raw-output '."rbd-entrance".version' /etc/goodrain/envs/rbd.json)
RBD_ENTRANCE="rainbond/rbd-entrance:$RBD_ENTRANCE_VER"

LOCAL_IP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')

log.info "local ip:$LOCAL_IP"

ADD_EN_IP=$(echo $ENTRANCE_IP | tr ',' '\n' | grep -v $LOCAL_IP | sort -u | xargs | tr ' ' ',')

log.info "need add othor entrance ip: $ADD_EN_IP"

OLD_EN_IP=$(cat /etc/goodrain/docker-compose.yaml | grep "api=" | awk -F '=' '{print $3}' | uniq | tr ';' '\n' | awk -F '[:/]' '{print $4}' | grep -v '127.0.0.1' | xargs | tr ' ' ',')

function check_config() {
    dest_md5=$(echo $ADD_EN_IP | tr ',' '\n' | sort -u | xargs | md5sum | awk '{print $1}')

    old_md5=$(echo $OLD_EN_IP | tr ',' '\n' | sort -u | xargs | md5sum | awk '{print $1}')

    log.info "new entrance md5sum: <$dest_md5>"
    log.info "old entrance md5sum: <$old_md5>"
    if [ "$dest_md5" == "$old_md5" ];then
        log.info "check entrance ok"
        return 0
    else
        log.info "check entrance failed, need reconf"
        return 1
    fi

}

function compose::config_update() {
    YAML_FILE=/etc/goodrain/docker-compose.yaml
    mkdir -pv `dirname $YAML_FILE`
    if [ ! -f "$YAML_FILE" ];then
        echo "version: '2.1'" > $YAML_FILE
    fi
    dc-yaml -f $YAML_FILE -u -
}

function write_entrance_config() {
    log.info "write_entrance_config"
    #ENTRANCE_NODE=()
    for entrance_node in $(echo $ADD_EN_IP | tr ',' ' ' | sort -u)
    do
        ENTRANCE_INFO=";http://$entrance_node:10002"
        echo "$ENTRANCE_INFO" >> /tmp/entrance
    done
    ENTRANCE_NODE=$(cat /tmp/entrance | sort -u | xargs | tr -d " ")
        compose::config_update << EOF
services:
  rbd-entrance:
    image: $RBD_ENTRANCE
    container_name: rbd-entrance
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
    volumes:
      - /etc/goodrain/kubernetes:/etc/goodrain/kubernetes
    command:
      - --plugin-name=nginx
      - --plugin-opts=httpapi=http://127.0.0.1:10002$ENTRANCE_NODE
      - --plugin-opts=streamapi=http://127.0.0.1:10002$ENTRANCE_NODE
      - --run-mode=sync
      - --kube-conf=/etc/goodrain/kubernetes/admin.kubeconfig
      - --log-level=info
EOF
    dc-compose up -d

    rm -rf /tmp/entrance
}

function run() {
    log.info "setting entrance config"
    check_config || (
        write_entrance_config
    )

    log.stdout '{ 
            "status":[ 
            { 
                "name":"update_entrance", 
                "condition_type":"UPDATE_ENTRANCE", 
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