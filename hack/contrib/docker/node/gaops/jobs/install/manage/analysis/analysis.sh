#!/bin/bash

REPO_VER=$1
REGION_TAG=${2:-cloudbang}

DALARAN_CEP=hub.goodrain.com/dc-deploy/cep_dalaran:${1:-3.4.1}
CEP_HBASE=hub.goodrain.com/dc-deploy/cep_hbase:${1:-3.4.1}
CEP_OPENTSDB=hub.goodrain.com/dc-deploy/cep_opentsdb:${1:-3.4.1}
CEP_SERVER=hub.goodrain.com/dc-deploy/cep_server:${1:-3.4.1}
CEP_RECORDER=hub.goodrain.com/dc-deploy/acp_labor:${1:-3.4.1}
LOGTRANSFER=hub.goodrain.com/dc-deploy/cep_logtransfer:${1:-3.4.1}
CEP_PRISM=hub.goodrain.com/dc-deploy/cep_prism:${1:-3.4.1}

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

function prepare() {
    log.info "prepare analysis for RBD"
}

function install_dalaran_cep() {
    log.info "setup dalaran"
        log.info "setup dalaran_cep"
    image::exist $DALARAN_CEP || (
        log.info "pull image: $DALARAN_CEP "
        image::pull $DALARAN_CEP || (exit 1)
    )

    compose::config_update << EOF
services:
  cep_dalaran:
    image: $DALARAN_CEP
    container_name: cep_dalaran
    environment:
      ZMQ_BIND_SUB: tcp://0.0.0.0:9441
      ZMQ_BIND_PUB: tcp://0.0.0.0:9442
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

function install_hbase() {
    log.info "setup hbase"

    image::exist $CEP_HBASE || (
        log.info "pull image: $CEP_HBASE"
        image::pull $CEP_HBASE || (exit 1)
    )

    compose::config_update << EOF
services:
  cep_hbase:
    image: $CEP_HBASE
    container_name: cep_hbase
    volumes:
      - /data/hbase:/data
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d

    log.info "Waiting for hbase started..."
    i=0
    b=''
    while((i<=60))
    do
        b=.$b
        printf "Waiting for hbase started %-50s\r" $b
        docker exec cep_hbase curl -s -m 1 --connect-timeout 1 http://127.0.0.1:60010/master-status 2>&1 >/dev/null && break
        sleep 1
        ((i+=1))
    done
    echo

    docker exec cep_hbase ./bin/create_table.sh
}

function install_opentsdb() {
    log.info "setup opentsdb"
    image::exist $CEP_OPENTSDB || (
        log.info "pull image: $CEP_OPENTSDB"
        image::pull $CEP_OPENTSDB || (exit 1)
    )

    compose::config_update << EOF
services:
  cep_opentsdb:
    image: $CEP_OPENTSDB
    container_name: cep_opentsdb
    volumes:
      - /data/hbase:/data
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
    depends_on:
      - cep_hbase
EOF

    dc-compose up -d

}

function install_cepserver() {
    log.info "setup cepserver"
    image::exist $CEP_SERVER || (
        log.info "pull image: $CEP_SERVER"
        image::pull $CEP_SERVER || (exit 1)
    )

    compose::config_update << EOF
services:
  cep_server:
    image: $CEP_SERVER
    container_name: cep_server
    environment:
      INSTANCE_LOCK_NAME: cep_server
      ZMQ_SUB_FROM: tcp://127.0.0.1:9442
      ZMQ_PUB_TO: tcp://127.0.0.1:9441
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
    depends_on:
      - cep_dalaran
EOF

    dc-compose up -d
}

function install_cep_recorder() {
    log.info "setup cep_recorder "
    
    image::exist $CEP_RECORDER || (
        log.info "pull image: $CEP_RECORDER"
        image::pull $CEP_RECORDER || (exit 1)
    )

    compose::config_update << EOF
services:
  cep_recorder:
    image: $CEP_RECORDER
    container_name: cep_recorder
    environment:
      REGION_TAG: $REGION_TAG
    volumes:
      - /etc/goodrain/labor.py:/app/labor/etc/regions/$REGION_TAG.py
    command: bin/cep_recorder.pyc
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

function update_console() {
    log.info "update console"
    if [ -f "/etc/goodrain/console.py" ];then
        sed -i "s/\"Monitor_Control\": False/\"Monitor_Control\": True/g"  /etc/goodrain/console.py
        dc-compose up -d
        docker exec acp_web /app/console_manage migrate
        dc-compose restart acp_web
    else
        log.error "Not found console.py"
        exit 1
    fi
}

function install_logtransfer() {
    log.info "install logtransfer"
    image::exist $LOGTRANSFER || (
        log.info "pull image: $LOGTRANSFER"
        image::pull $LOGTRANSFER || (exit 1)
    )

    compose::config_update << EOF
services:
  cep_logtransfer:
    image: $LOGTRANSFER
    container_name: cep_logtransfer
    environment:
      ZMQ_PUB_TO: tcp://127.0.0.1:9441
      LOG_DIR: /data/openrestry/logs
    volumes:
      - /data/openrestry/logs:/data/openrestry/logs
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

function compute_install_prism() {
    log.info "install prism"
        image::exist $CEP_PRISM || (
        log.info "pull image: $CEP_PRISM"
        image::pull $CEP_PRISM || (exit 1)
    )
    # ZMQ_PUB_TO set dalaran_cep host
    compose::config_update << EOF
services:
  cep_prism:
    image: $CEP_PRISM
    container_name: cep_prism
    environment:
      ZMQ_BIND_SUB: tcp://172.30.42.1:7388
      ZMQ_PUB_TO: tcp://127.0.0.1:9441
      ZMQ_IO_THREADS: 2
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

function run() {
    install_dalaran_cep
    install_hbase
    install_opentsdb
    install_cepserver
    install_cep_recorder

    update_console
    install_logtransfer
    
    if [ -f "/etc/goodrain/envs/kubelet.sh" ];then
        compute_install_prism
    fi
}

case $1 in
    *)
        prepare
        run
    ;;
esac
