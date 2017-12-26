#!/bin/bash

set -o pipefail

MYSQL_EXPAND=${1:-0} #是否支持扩容
MYSQL_USER=$2
MYSQL_PASSWD=$3
MYSQL_HOST=${4:-127.0.0.1}
MYSQL_PORT=${5:-3306}

MYSQL_VAR=$#

RBD_IMAGE_DB_NAME=$(jq --raw-output '."rbd-db".name' /etc/goodrain/envs/rbd.json)
RBD_IMAGE_DB_VERSION=$(jq --raw-output '."rbd-db".version' /etc/goodrain/envs/rbd.json)
RBD_DIM=$RBD_IMAGE_DB_NAME:$RBD_IMAGE_DB_VERSION

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

function compose::config_remove() {
    service_name=$1
    YAML_FILE=/etc/goodrain/docker-compose.yaml
    mkdir -pv `dirname $YAML_FILE`
    if [ -f "$YAML_FILE" ];then
        dc-yaml -f $YAML_FILE -d $service_name
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

function compose::confict() {
    service_name=$1
    compose::config_remove $service_name
    remove_ctn_ids=$(docker ps --filter label=com.docker.compose.service=${service_name} -q)
    if [ -n "$remove_ctn_ids" ];then
        log.info "remove containers create by docker-compose for service $service_name "
        for cid in $(echo $remove_ctn_ids)
        do
            docker kill $cid
            docker rm $cid
        done
    fi
}

function package::is_listen_port() {
    port=$1
    listen_info=($(netstat -tunlp | grep $port | grep tcp | awk '{print $1" "$4}'))
    protocl=${listen_info[0]}

    if [ "$protocl" == "tcp6" ]; then
        log.info "port $port is listening with tcp6"
    else
        listen_address=${listen_info[1]%:*}
        if [ -z "$listen_address" ]; then
            log.error "port $port is not on listening"
            return 1
        fi

        if [ "$listen_address" == "127.0.0.1" ];then
            log.error "port $port is listening on localhost"
            return 2
        fi

        log.info "port $port is listening on $listen_address"
    fi
    
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

function check_mysql_admin() {
    
    check_result=$(docker exec rbd-db mysql -u $MYSQL_USER -P $MYSQL_PORT -p$MYSQL_PASSWD \
    -e "select user,host,grant_priv from mysql.user where user='"$MYSQL_USER"'")

    if [ -z "$check_result" ];then
        log.error "authenticate with user '"$MYSQL_USER"' failed!"
        return 1
    else
        log.info "authenticate with user '"$MYSQL_USER"' success!"
        return 0
    fi
}

function prepare() {

    log.info "install docker plugins db"
    log.info "prepare db"
    mkdir -pv /data/db

    if [ $MYSQL_VAR -eq 1 ];then
        # 需要手动生成密码和用户名
        MYSQL_USER="write1"
        MYSQL_PORT="3306"
        MYSQL_PASSWD=$(echo $((RANDOM)) | base64 | md5sum | cut -b 1-8)
        echo "$MYSQL_PASSWD" > /data/.db_passwd
    elif [ $MYSQL_VAR -eq 5 ];then
        log.info "$MYSQL_USER $MYSQL_PASSWD $MYSQL_HOST $MYSQL_PORT"
    else
        log.error "mysql configure parameter error, VERSION MYSQL_USER MYSQL_PASSWD MYSQL_HOST MYSQL_PORT"
        log.status '{
            "status":[ 
                { 
                    "name":"db_configure_parameter_error", 
                    "condition_type":"DB_PARAMETER_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Failure",
                "type":"install"
        }'
        exit 1
    fi
}

function one_db() {

    
    log.info "install db"
    compose::confict mysql
   
    image::exist $RBD_DIM || (
        log.info "pull image: $RBD_DIM"
        image::pull $RBD_DIM || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_db_faild", 
                    "condition_type":"DOCKER_PULL_DB_ERROR", 
                    "condition_status":"False"
                } 
                ],
                "exec_status":"Failure",
                "type":"install"
                }'
            exit 1
        )
    )

    compose::confict mariadb
    compose::config_update << EOF
services:
  rbd-db:
    image: $RBD_DIM
    container_name: rbd-db
    volumes:
      - /data/db:/data
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d

    log.info "waiting for db startup"
    while true
    do
        if package::is_listen_port $MYSQL_PORT;then
            break
        else
            sleep 5
        fi
    done

    check_mysql_admin || (
        log.info "create admin user"
        
        docker exec rbd-db mysql -e "grant all on *.* to $MYSQL_USER@'%' identified by '"$MYSQL_PASSWD"' with grant option; flush privileges"
        docker exec rbd-db mysql -e "delete from mysql.user where user=''; flush privileges"
        
        log.info "recheck admin_user"
        check_mysql_admin
    )
    MYSQL_HOST=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')

    docker exec rbd-db mysql -e "CREATE DATABASE IF NOT EXISTS region DEFAULT CHARSET utf8 COLLATE utf8_general_ci;"
    docker exec rbd-db mysql -e "CREATE DATABASE IF NOT EXISTS console DEFAULT CHARSET utf8 COLLATE utf8_general_ci;"

    log.info "create database region, console success!"
    #log.stdout "{'db_type':'mysql','info':['MYSQL_USER':'"$MYSQL_USER"','MYSQL_PASSWD':'"$MYSQL_PASSWD"','MYSQL_HOST':'"$MYSQL_HOST"','MYSQL_PORT':'"$MYSQL_PORT"']}"
    MYSQL_EXPAND=1
    log.stdout '{ 
            "global":{
              "DB_MODE":"'mysql'",
              "DB_USER":"'$MYSQL_USER'",
              "DB_PASSWD":"'$MYSQL_PASSWD'",
              "DB_HOST":"'$MYSQL_HOST'",
              "DB_PORT":"'$MYSQL_PORT'",
              "DB_EXPAND":"'$MYSQL_EXPAND'"
            },
            "status":[ 
            { 
                "name":"install_db", 
                "condition_type":"INSTALL_DB", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

function run(){
    if [[ $MYSQL_EXPAND -eq 0 ]];then
        log.info "init db"
        prepare
        one_db
    else
        
        log.info "pass init db"
        log.stdout '{
            "status":[ 
            { 
                "name":"install_db_manage", 
                "condition_type":"INSTALL_DB_MANAGE", 
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
        run
        ;;
esac