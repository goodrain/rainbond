#!/bin/bash

REPO_VER=$1
EX_DOMAIN=$2
RBD_REPO_EXPAND=${3:-0}
LANG_SERVER=${4:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}
MAVEN_SERVER=${5:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}
DNS_SERVER=${6:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}
HUB_SERVER=${7:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}



#EXIP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')
EXIP=$(cat /etc/goodrain/envs/.exip  | awk '{print $1}')

RBD_IMAGE_DNS_NAME=$(jq --raw-output '."rbd-dns".name' /etc/goodrain/envs/rbd.json)
RBD_IMAGE_DNS_VERSION=$(jq --raw-output '."rbd-dns".version' /etc/goodrain/envs/rbd.json)

RBD_DNS=$RBD_IMAGE_DNS_NAME:$RBD_IMAGE_DNS_VERSION

RBD_REGISTRY="rainbond/rbd-registry:2.3.1"

RBD_IMAGE_REPO_NAME=$(jq --raw-output '."rbd-repo".name' /etc/goodrain/envs/rbd.json)
RBD_IMAGE_REPO_VERSION=$(jq --raw-output '."rbd-repo".version' /etc/goodrain/envs/rbd.json)

RBD_REPO=$RBD_IMAGE_REPO_NAME:$RBD_IMAGE_REPO_VERSION


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
        [ -z "$EXIP" ] && (
                EXIP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')
        )
        
        log.info "domain resolve: $EXIP."

        docker pull hub.goodrain.com/dc-deploy/archiver:domain
        #docker run -it --rm hub.goodrain.com/dc-deploy/archiver:domain init --ip $IP > /tmp/domain.log
        [ -f "/data/.domain.log" ] && echo "" > /data/.domain.log || touch /data/.domain.log
        
        docker run  --rm -v /data/.domain.log:/tmp/domain.log hub.goodrain.com/dc-deploy/archiver:domain init --ip $EXIP > /tmp/do.log
        if [ $? -eq 0 ];then
            EX_DOMAIN=$(cat /data/.domain.log)
        else
            log.error "Domain name not generated"
        fi
        log.info "DOMAIN:$EX_DOMAIN"
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
    log.info "prepare base plugins"

    # 待测试管理节点扩容
    #sys::path_mounted /grdata || exit 3 
    log.info "add rain user"
    add_user
    log.info "config domain info"
    make_domain
}

function install_dns() {

    log.info "setup dns"

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
    log.info "setup registry"

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
    log.info "setup repo"

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

function run() {
    
    log.info "setup RBD base plugins"
    install_dns
    install_registry
    if [ $RBD_REPO_EXPAND -eq 0 ];then
        install_repo
        RBD_REPO_EXPAND=1
        log.stdout '{ 
                "global":{
                "DOMAIN":"'$EX_DOMAIN'",
                "DNS_SERVER":"'$DNS_SERVER',",
                "HUB_SERVER":"'$HUB_SERVER',",
                "RBD_REPO_EXPAND":"'$RBD_REPO_EXPAND'",
                "LANG_SERVER":"'$LANG_SERVER'",
                "MAVEN_SERVER":"'$MAVEN_SERVER'"
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
    else
        log.stdout '{ 
                "global":{
                "DNS_SERVER":"'$DNS_SERVER',",
                "HUB_SERVER":"'$HUB_SERVER',"
                },
                "status":[ 
                { 
                    "name":"install_base_plugins_manage", 
                    "condition_type":"INSTALL_BASE_PLUGINS_MANAGE", 
                    "condition_status":"True"
                } 
                ], 
                "exec_status":"Success",
                "type":"install"
                }'
    fi
}

case $1 in
    * )
        prepare
        run
        ;;
esac