#!/bin/bash

REPO_VER=$1
EX_DOMAIN=$2
IP=${3:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}

RBD_DNS="rainbond/rbd-dns:$REPO_VER"
RBD_REGISTRY="rainbond/rbd-registry:2.3.1"
RBD_REPO="rainbond/rbd-repo:$REPO_VER"
RBD_DALARAN="rainbond/rbd-dalaran:$REPO_VER"
RBD_ENTRANCE="rainbond/rbd-entrance:$REPO_VER"

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

function sys::path_mounted() {
    dest_dir=$1
    if [ ! -d "$dest_dir" ]; then
        log.info "dir $dest_dir not exist"
        return 1
    fi
    
    df -h | grep $dest_dir >/dev/null && (
        log.info "$dest_dir already mounted"
        return 0
    ) || (
        log.error "$dest_dir not mounted"
        return 1
    )
}

function compose::config_update() {
    YAML_FILE=/etc/goodrain/docker-compose.yaml
    mkdir -pv `dirname $YAML_FILE`
    if [ ! -f "$YAML_FILE" ];then
        echo "version: '2.1'" > $YAML_FILE
    fi
    dc-yaml -f $YAML_FILE -u -
}

function image::exist() {
    IMAGE=$1
    docker images  | sed 1d | awk '{print $1":"$2}' | grep $IMAGE >/dev/null 2>&1
    if [ $? -eq 0 ];then
        log.info "image $IMAGE exists"
        return 0
    else
        log.error "image $IMAGE not exists"
        return 1
    fi
}

function image::pull() {
    IMAGE=$1
    docker pull $IMAGE
    if [ $? -eq 0 ];then
        log.info "pull image $IMAGE success"
        return 0
    else
        log.info "pull image $IMAGE failed"
        return 1
    fi
}

function add_user() {
    grep rain /etc/group >/dev/null 2>&1 || groupadd -g 200 rain
    id rain >/dev/null 2>&1 || (
        useradd -m -s /bin/bash -u 200 -g 200 rain
        echo "rain ALL = (root) NOPASSWD:ALL" > /etc/sudoers.d/rain
        chmod 0440 /etc/sudoers.d/rain
    )
    log.info "add_user ok"
}

function make_domain() {
    if [ -z "$EX_DOMAIN" ];then
        IP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')
        docker pull hub.goodrain.com/dc-deploy/archiver:domain
        #docker run -it --rm hub.goodrain.com/dc-deploy/archiver:domain init --ip $IP > /tmp/domain.log
        [ -f "/tmp/domain.log" ] && echo "" > /tmp/domain.log || touch /tmp/domain.log
         
        docker run --rm -v /tmp/domain.log:/tmp/domain.log hub.goodrain.com/dc-deploy/archiver:domain init --ip $IP > /tmp/do.log
        if [ $? -eq 0 ];then
            EX_DOMAIN=$(cat /tmp/domain.log)
        else
            touch /tmp/fuck
        fi
        #docker rmi hub.goodrain.com/dc-deploy/archiver:domain
    fi

    if [ "$EX_DOMAIN" = "" ];then
        log.stdout '{
            "status":[ 
            { 
                "name":"EX_DOMAIN_ERROR", 
                "condition_type":"EX_DOMAIN_ERROR", 
                "condition_status":"False"
            } 
            ],
            "type":"install"
            }'
        exit 1
    fi
    
}

function prepare() {
    log.section "prepare base plugins"

    # 待测试管理节点扩容
    #sys::path_mounted /grdata || exit 3 
    log.info "add rain user"
    add_user
    log.info "config domain info"
    make_domain
}

function install_dns() {

    log.section "setup dns"

        image::exist $RBD_DNS || (
        log.info "pull image: $RBD_DNS"
        image::pull $RBD_DNS || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_dns", 
                    "condition_type":"DOCKER_PULL_DNS_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )

    compose::config_update << EOF
services:
  rbd-dns:
    image: $RBD_DNS
    container_name: rbd-dns
    environment:
      - KUBEURL=http://127.0.0.1:8181
      - SKYDNS_DOMAIN=goodrain.me
      - RECORD_1=goodrain.me:172.30.42.1
      - RECORD_2=lang.goodrain.me:172.30.42.1
      - RECORD_3=maven.goodrain.me:172.30.42.1
      - RECORD_4=config.goodrain.me:172.30.42.1
      - RECORD_5=console.goodrain.me:172.30.42.1
      - RECORD_6=region.goodrain.me:172.30.42.1
      - RECORD_7=kubeapi.goodrain.me:172.30.42.1
      - RECORD_8=download.goodrain.me:172.30.42.1
    logging:
      driver: json-file
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d
}

function install_registry() {
    log.section "setup registry"

    image::exist $RBD_REGISTRY || (
        log.info "pull image: $RBD_REGISTRY"
        image::pull $RBD_REGISTRY || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_registry", 
                    "condition_type":"DOCKER_PULL_REGISTRY_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )

    compose::config_update << EOF
services:
  rbd-hub:
    image: $RBD_REGISTRY
    container_name: rbd-hub
    volumes:
      - /grdata/services/registry/:/var/lib/registry
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d

}


function install_repo() {
    log.section "setup repo"

    image::exist $RBD_REPO || (
        log.info "pull image: $RBD_REPO"
        image::pull $RBD_REPO || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_repo", 
                    "condition_type":"DOCKER_PULL_REPO_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )

    compose::config_update << EOF
services:
  rbd-repo:
    image: $RBD_REPO
    container_name: rbd-repo
    environment:
      INSTANCE_LOCK_NAME: artifactory
    volumes:
    - /grdata/services/artifactory5:/var/opt/jfrog/artifactory
    logging:
      driver: json-file
      options:
        max-size: 50m
        max-file: '3'
    network_mode: host
    restart: always
EOF
    dc-compose up -d
}

function install_dalaran() {
    log.section "setup dalaran_service"
    
    image::exist $RBD_DALARAN || (
        log.info "pull image: $RBD_DALARAN"
        image::pull $RBD_DALARAN || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_dalaran", 
                    "condition_type":"DOCKER_PULL_DALARAN_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )

    compose::config_update << EOF
services:
  rbd-dalaran:
    image: $RBD_DALARAN
    container_name: rbd-dalaran
    environment:
      ZMQ_BIND_SUB: tcp://0.0.0.0:9341
      ZMQ_BIND_PUB: tcp://0.0.0.0:9342
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d
}

function install_entrance() {
    log.section "setup entrance"
    
    image::exist $RBD_ENTRANCE || (
        log.info "pull image: $RBD_ENTRANCE"
        image::pull $RBD_ENTRANCE || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_entrance", 
                    "condition_type":"DOCKER_PULL_ENTRANCE_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )

    [ -f "/etc/goodrain/kubernetes/admin.kubeconfig" ] || (
        [ -f "/etc/goodrain/kubernetes/kubeconfig" ] && cp /etc/goodrain/kubernetes/kubeconfig /etc/goodrain/kubernetes/admin.kubeconfig
    )

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
      - --plugin-opts=httpapi=http://127.0.0.1:10002
      - --plugin-opts=streamapi=http://127.0.0.1:10002
      #- --token=
      - --kube-conf=/etc/goodrain/kubernetes/admin.kubeconfig
      - --log-level=info
EOF
    dc-compose up -d

}

function run() {
    
    log.section "setup RBD base plugins"
    install_dns
    install_registry
    install_repo
    install_dalaran
    install_entrance
    

    log.stdout '{ 
            "global":{
              "DOMAIN":"'$EX_DOMAIN'",
              "DNS_SERVER":"'$IP'"
            },
            "status":[ 
            { 
                "name":"install_base_plugins", 
                "condition_type":"INSTALL_BASE_PLUGINS", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

case $1 in
    * )
        prepare
        run
        ;;
esac