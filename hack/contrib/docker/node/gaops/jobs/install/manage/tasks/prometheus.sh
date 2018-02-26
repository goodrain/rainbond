#!/bin/bash 

REGION_TAG=${1:-cloudbang}
RROM_EXPAND=${2:-0}

PROM_VER=$(jq --raw-output '."prometheus".version' /etc/goodrain/envs/rbd.json)
PROM="prom/prometheus:$PROM_VER"

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

function prepare() {
    [ -d "/grdata/services/prometheus/data" ] || (
        log.info "create /grdata/services/prometheus/data"
        mkdir -p /grdata/services/prometheus/data
        chmod 777 /grdata/services/prometheus/data
    )
    log.info "prepare for promethues"
}

function install_prometheus() {
    
    image::exist $PROM || (
        log.info "pull image: $PROM"
        image::pull $PROM || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_prometheus", 
                    "condition_type":"DOCKER_PULL_PROMETHEUS_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "exec_status":"Failure",
                "type":"install"
                }'
            exit 1
        )
    )

    [ -f "/etc/goodrain/prometheus/prometheus.yml" ] && (
        log.info "prometheus.yml exist"
    ) || (
        mkdir -p /etc/goodrain/prometheus
        log.info "create /etc/goodrain/prometheus/prometheus.yml"
        cat > /etc/goodrain/prometheus/prometheus.yml <<EOF
global:
  scrape_interval:     5s
  evaluation_interval: 30s

scrape_configs:
- job_name: prometheus

  honor_labels: true

  metrics_path: '/metrics'
  static_configs:
  - targets: ['localhost:9999']
    labels:
      datacenter: $REGION_TAG

- job_name: event_log

  honor_labels: true

  static_configs:
  - targets: ['127.0.0.1:6363']
    labels:
      component: acp_event_log

- job_name: entrance
  honor_labels: true

  static_configs:
  - targets: ['127.0.0.1:6200']
    labels:
      component: acp_entrance


- job_name: APA

  metrics_path: '/app/metrics'
  static_configs:
  - targets: ['127.0.0.1:6100']
    labels:
      datacenter: $REGION_TAG

- job_name: Node

  metrics_path: '/node/metrics'
  static_configs:
  - targets: ['127.0.0.1:6100']
    labels:
      datacenter: $REGION_TAG
EOF
    )

    compose::config_update << EOF
services:
  prometheus:
    image: $PROM
    container_name: prometheus
    volumes:
        - /grdata/services/prometheus/data:/prometheusdata
        - /etc/goodrain/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
    command: --web.listen-address=":9999" --storage.tsdb.path="/prometheusdata" --storage.tsdb.retention=7d --config.file="/etc/prometheus/prometheus.yml"
    logging:
        driver: "json-file"
        options:
          max-size: "50m"
          max-file: "3"
    network_mode: "host"
    restart: always
EOF
    dc-compose up -d prometheus

}

function run_install() {
    
    log.info "setup prometheus"
    install_prometheus
    dc-compose ps  | grep prometheus | grep Up
     _EXIT=1
    for ((i=1;i<=3;i++ )); do
        sleep 3
        log.info "retry $i get prometheus "
        dc-compose ps | grep "prometheus" && export _EXIT=0 && break
    done
    if [ $_EXIT -eq 0 ];then
        log.info "Install prometheus Successful."
        PROM_EXPAND=1
        log.stdout '{ 
            "global":{
                    "PROM_EXPAND":"'$PROM_EXPAND'"
                 },
                "status":[ 
                { 
                    "name":"install_prometheus", 
                    "condition_type":"INSTALL_PROMETHEUS", 
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
                    "name":"install_prometheus", 
                    "condition_type":"INSTALL_PROMETHEUS_FAILED", 
                    "condition_status":"False"
                } 
                ],
                "exec_status":"Failure",
                "type":"install"
                }'
    fi
}

function run(){
    if [[ $PROM_EXPAND -eq 0 ]];then
        prepare
        run_install
    else
        log.info "pass install prom"
        log.stdout '{
            "status":[ 
            { 
                "name":"install_prom_manage", 
                "condition_type":"INSTALL_PROM_MANAGE", 
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