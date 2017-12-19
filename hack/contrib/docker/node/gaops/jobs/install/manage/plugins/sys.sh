#!/bin/bash

REPO_VER=$1
EX_DOMAIN=$2 #.rw9h0.goodrain.org
MYSQL_USER=${3:-write1}
MYSQL_PASSWD=$4
MYSQL_HOST=$5
MYSQL_PORT=$6
HOST_IP=$7 # 此节点ip
CUR_NET=${8:-calico} #calico/midonet
REGION_TAG=${9:-cloudbang}
MYSQL_DB="region"

[ -z "$MYSQL_USER" ] && MYSQL_USER="write1"
[ -z "$MYSQL_PASSWD" ] && MYSQL_PASSWD=$(cat /data/.db_passwd) || (
    if [ "$MYSQL_PASSWD" != "$(cat /data/.db_passwd)" ];then
        MYSQL_PASSWD=$(cat /data/.db_passwd)
    fi
)
[ -z "REGION_TAG" ] && REGION_TAG=cloudbang

RBD_WEB="rainbond/rbd-app-ui:$REPO_VER"
RBD_WORKER="rainbond/rbd-worker:$REPO_VER"
RBD_CHAOS="rainbond/rbd-chaos:$REPO_VER"
RBD_SLOGGER="rainbond/rbd-slogger:$REPO_VER"
RBD_API="rainbond/rbd-api:$REPO_VER"
RBD_LB="rainbond/rbd-lb:$REPO_VER"
RBD_EVENTLOG="rainbond/rbd-eventlog:$REPO_VER"
RBD_MQ="rainbond/rbd-mq:$REPO_VER"

export KUBE_SHARE_DIR="/grdata/services/k8s"

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

function prepare() {
    log.info "prepare base plugins"

    # 待测试管理节点扩容
    #sys::path_mounted /grdata || exit 3 

    [ -d "/grdata/tenant" ] || (
        mkdir /grdata/tenant
        chown rain.rain /grdata/tenant
    )

    [ -d "/etc/goodrain/openresty" ] || (
        mkdir /etc/goodrain/openresty
        chown rain.rain /etc/goodrain/openresty
    )

}

function image::done() {
    #image::exist $1 || (
    #    log.info "pull image: $1"
        image::pull $1 || (
            log.stdout '{
            "status":[ 
            { 
                "name":"docker_pull_'$1'", 
                "condition_type":"DOCKER_PULL_'$1'_ERROR", 
                "condition_status":"Flase"
            } 
            ],
            "type":"install"
            }'
            exit 1
        )
    #)
}

function write_region_api_cfg() {
    cat <<EOF > /etc/goodrain/region_api.py
# -*- coding: utf8 -*-
DEFAULT_HANDLERS = ['zmq_handler']

ZMQ_LOG_ADDRESS = 'tcp://127.0.0.1:9341'

REST_FRAMEWORK = {
    'DEFAULT_PERMISSION_CLASSES': (),
    'PAGE_SIZE': 10
}

DATABASES = {
    'default': {
        'ENGINE': 'django.db.backends.mysql',
        'NAME': 'region',
        'USER': '$MYSQL_USER',
        'PASSWORD': '$MYSQL_PASSWD',
        'HOST': '${MYSQL_HOST:-127.0.0.1}',
        'PORT': '${MYSQL_PORT:-3306}',
    },
}


BEANSTALKD = {
    "host": "127.0.0.1",
    "port": 11300,
    "tube": "default"
}


KEYSTONE_API = {
    'url': 'http://$HOST_IP:35357/v2.0',
    'token': 'empty',
    'apitype': 'keystone api'
}


MIDONET_API = {
    'url': 'http://:/midonet-api',
    'username': 'admin',
    'password': 'unknown',
    'project_id': 'admin',
    'provider_router_id': 'unknown',
    'apitype': 'midonet api'
}


KUBERNETES_API = {
    'url': 'http://127.0.0.1:8181/api/v1',
    'apitype': 'kubernetes api'
}

KUBERNETES_JOB_API = {
    'url': 'http://127.0.0.1:8181/apis/extensions/v1beta1',
    'apitype': 'kubernetes job api'
}


OPENTSDB_API = {
    'url': 'http://$HOST_IP:4242/api',
    'apitype': 'opentsdb api'
}

ETCD = {
    "host": "127.0.0.1",
    "port": 4001
}

EX_DOMAIN = {
    "$REGION_TAG": ".$EX_DOMAIN",
}


SLUG_SERVER = "172.30.42.1:8584"

CONTAINER_HOST_DNS = False

REVERSE_DEPEND_SERVICE = {
    "service-collector": True,
    "logstash": True
}


LB_NGINX = {
    'enabled': True,
}


CUR_NET = "$CUR_NET"


CONTAINER_MONITOR_API = {
    'url': '',
    'apitype': 'container monitor api'
}

FLOCKER_API = {
    'host': '',
    'port': 4523,
    'url': '/v1/configuration/datasets',
    'KEY_FILE': "/etc/flocker/scio01.key",
    'CERT_FILE': "/etc/flocker/scio01.crt",
    'CA_FILE': "/etc/flocker/cluster.crt"
}
EOF
}

function sync_certificates() {
    
    [ ! -d "/etc/goodrain/kubernetes" ] && mkdir -p /etc/goodrain/kubernetes || (
        [ ! -f "/etc/goodrain/kubernetes/admin.kubeconfig" ] && (
            cp /grdata/kubernetes/admin.kubeconfig /etc/goodrain/kubernetes/admin.kubeconfig
            chmod 644 /etc/goodrain/kubernetes/admin.kubeconfig
        ) || (
            log.info ""
        )
    )
    log.info "sync_certificates success!"
}

function grctl_check() {
    #which grctl >/dev/null 2>&1 || \
    #docker run --rm -v /var/run/docker.sock:/var/run/docker.sock hub.goodrain.com/dc-deploy/archiver grctl

    [ ! -f "/etc/goodrain/grctl.json" ] && (

    cat >>/etc/goodrain/grctl.json <<EOF
{
    "RegionMysql": {
        "URL": "${MYSQL_HOST:-127.0.0.1}:${MYSQL_PORT:-3306}",
        "User": "$MYSQL_USER",
        "Pass": "$MYSQL_PASSWD",
        "Database": "$MYSQL_DB"
    },
    "Kubernets": {
        "Master": "http://127.0.0.1:8181"
    },
    "RegionAPI": {
        "URL": "http://region.goodrain.me:8888"
    },
    "DockerLogPath": "/data/docker_logs/"
}
EOF
    )
}

function install_api(){
    write_region_api_cfg
    sync_certificates

    compose::config_update << EOF
services:
  rbd-api:
    image: $RBD_API
    container_name: rbd-api
    environment:
      REGION_TAG: $REGION_TAG
      EX_DOMAIN: $EX_DOMAIN
      LicenseSwitch: "off"
    volumes:
      - /etc/goodrain/region_api.py:/app/region_api/conf/$REGION_TAG.py
      - /etc/goodrain/kubernetes:/etc/goodrain/kubernetes
      - /grdata:/grdata
      - /data/docker_logs:/data/docker_logs
    command: --log-level=debug --mysql="$MYSQL_USER:$MYSQL_PASSWD@tcp(${MYSQL_HOST:-127.0.0.1}:${MYSQL_PORT:-3306})/$MYSQL_DB"
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    grctl_check

}

function web_write_cfg() {
    cat <<EOF > /etc/goodrain/console.py
import os

DEBUG = True

TEMPLATE_DEBUG = False

ZMQ_LOG_ADDRESS = 'tcp://127.0.0.1:9341'

DEFAULT_HANDLERS = ['zmq_handler']

EMAIL_BACKEND = 'django.core.mail.backends.smtp.EmailBackend'

EMAIL_HOST = 'xxxx.xxx.xxx'
EMAIL_PORT = 465
EMAIL_HOST_USER = 'xxx@xxx.com'
EMAIL_HOST_PASSWORD = 'xxxx'
EMAIL_USE_SSL = True

DISCOURSE_SECRET_KEY = 'xxxxx'

#ALLOWED_HOSTS = []

REGION_TOKEN = ""


WILD_DOMAIN = ".$EX_DOMAIN"


STREAM_DOMAIN = True


STREAM_DOMAIN_URL = {
    "$REGION_TAG": "10.80.86.19"
}


WILD_DOMAINS = {
    "$REGION_TAG": ".$EX_DOMAIN"
}

WILD_PORTS = {
    "$REGION_TAG": "80"
}


REST_FRAMEWORK = {
    'DEFAULT_PERMISSION_CLASSES': (),
    'DEFAULT_AUTHENTICATION_CLASSES': (),
    'PAGE_SIZE': 10
}


DATABASES = {
    'default': {
        'ENGINE': 'django.db.backends.mysql',
        'NAME': 'console',
        'USER': '${MYSQL_USER}',
        'PASSWORD': '${MYSQL_PASSWD}',
        'HOST': '${MYSQL_HOST:-127.0.0.1}',
        'PORT': ${MYSQL_PORT:-3306},
    },
}



REGION_SERVICE_API = [{
    'url': 'http://region.goodrain.me:8888',
    'apitype': 'region service',
    'region_name': '$REGION_TAG'
}]



WEBSOCKET_URL = {
    '$REGION_TAG': 'ws://:/websocket',
}



EVENT_WEBSOCKET_URL = {
    '$REGION_TAG': 'auto',
}


APP_SERVICE_API = {
    'url': 'http://app.goodrain.com:80',
    'apitype': 'app service'
}

REGION_RULE = {
    'dev': {'personal_money': 0.069, 'company_money': 0.276, 'personal_month_money': 50, 'company_month_money': 100},
}

REGION_FEE_RULE = {
    'dev': {'memory_money': 0.069, 'disk_money': 0.0041, 'net_money': 0.8},
}

SESSION_ENGINE = "django.contrib.sessions.backends.cached_db"

MODULES = {
    "Owned_Fee": False,
    "Memory_Limit": False,
    "GitLab_Project": False,
    "GitLab_User": False,
    "Git_Hub": False,
    "Git_Code_Manual": True,
    "Finance_Center": False,
    "Team_Invite": True,
    "Monitor_Control": False,
    "User_Register": True,
    "Sms_Check": False,
    "Email_Invite": True,
    "Package_Show": False,
    "RegionToken": False,
    "Add_Port": True,
    "License_Center": False,
    "WeChat_Module": False,
    "Docker_Console": True,
    "Publish_YunShi": False,
    "Publish_Service": False,
}

REGIONS = (
    {"name": "$REGION_TAG", "label": '$REGION_TAG', "enable": True},
)


# logo path
MEDIA_ROOT = '/data/media'

SN = '01d1S-WMrCLEKypQ_jCW78MEkB-LqhgMIvZIlK3x9vuS-WlUjMkUG5OK8OCe_4KvrfYptfyc8PWe7adI21D57JnbHMU7paNCLxu4xMCK3ACXO97LifX8EBpkJUdjv8AnK0uZ0qXkoe2t0KFr_3cKfsYyG7F--QniyVElkjp6UJTBqXFU5E88easFVqA4YP9ARCGdbcxlp3ga6rfMq1KlRPv3G73hN4diUvcoP_0aOLbD7v17cuWWRXTfIcP5d1JuDTOHc0z-lGjwVQj4iJesBS1QaD5YpgrsJXzKAvI01'

# log domain
LOG_DOMAIN = {
    "$REGION_TAG": "auto"
}


IS_OPEN_API = False

WECHAT_CALLBACK = {
    "console": "",
    "console_bind": "",
    "console_goodrain": "",
    "console_bind_goodrain": "",
    "index": "",
}


DOCKER_WSS_URL = {
    'is_wide_domain': True,
    'type': 'ws',
    '$REGION_TAG': 'auto',
}



OAUTH2_APP = {
    'CLIENT_ID': '"$license_client_id"',
    'CLIENT_SECRET': '"$license_client_secret"',
}
EOF


}

function install_app_ui() {
    log.info "setup app_ui"

    web_write_cfg

        compose::config_update << EOF
services:
  rbd-app-ui:
    image: $RBD_WEB
    container_name: rbd-app-ui
    environment:
      REGION_TAG: $REGION_TAG
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
    volumes:
      - /etc/goodrain/console.py:/etc/goodrain/console.py
      - /grdata/services/console:/data
EOF

    mkdir -pv /grdata/services/console && chown rain.rain /grdata/services/console
    dc-compose up -d
    docker exec rbd-app-ui python /app/ui/manage.py migrate
}

function install_worker() {

    log.info "setup worker"

    compose::config_update << EOF
services:
  rbd-worker:
    image: $RBD_WORKER
    container_name: rbd-worker
    environment:
      MYSQL_HOST: ${MYSQL_HOST:-127.0.0.1}
      MYSQL_PORT: ${MYSQL_PORT:-3306}
      MYSQL_USER: $MYSQL_USER
      MYSQL_PASSWORD: $MYSQL_PASSWD
      MYSQL_DATABASE: $MYSQL_DB
      K8S_MASTER: http://127.0.0.1:8181
      #  CONSOLE_TOKEN:
      CUR_NET: $CUR_NET
      EX_DOMAIN: $EX_DOMAIN
    volumes:
      - /etc/goodrain:/etc/goodrain
      - /grdata:/grdata
    command: --log-level=info --kube-config="/etc/goodrain/kubernetes/admin.kubeconfig" --mysql="$MYSQL_USER:$MYSQL_PASSWD@tcp(${MYSQL_HOST:-127.0.0.1}:${MYSQL_PORT:-3306})/$MYSQL_DB"
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

function chaos_write_cfg() {
        [ -d "/etc/goodrain/etc/chaos/" ] || mkdir -pv /etc/goodrain/etc/chaos/
    cat <<EOF > /etc/goodrain/etc/chaos/config.json
    {
    "region": {
        "url": "http://region.goodrain.me:8888",
        "token": ""
    },
    "DEFAULT_HANDLERS": ["zmq_handler"],
    "EVENT_LOG_ADDRESS": "tcp://127.0.0.1:6366",
    "etcd": {
        "host": "127.0.0.1",
        "port": 2379
    },
    "userconsole": {
        "url": "http://console.goodrain.me",
        "token": ""
    },

"zmq": {
    "service_pub": {
        "address": "tcp://127.0.0.1:9341"
    }
  },

    "CLOUD_ASSISTANT": "256uw1474184267",
    "publish": {
    "slug": {
        "slug_path": "/grdata/build/tenant/",
        "curr_region_path": "/grdata/build/tenant/",
        "curr_region_dir": "app_publish/",
        "all_region_ftp": false,
        "all_region_ftp_host": "139.196.88.57",
        "all_region_ftp_port": "10021",
        "all_region_username": "commitity",
        "all_region_password": "commitity",
        "all_region_namespace": "app-publish/",
        "oss_ftp": true,
        "oss_ftp_host": "139.196.88.57",
        "oss_ftp_port": "10021",
        "oss_username": "commitity",
        "oss_password": "commitity",
        "oss_namespace": "app-publish/"
    },
    "image": {
        "curr_registry": "goodrain.me",
        "all_region_image": false,
        "all_registry": "oss.goodrain.me",
        "oss_image": true,
        "oss_host": "hub.goodrain.com",
        "oss_namespace": "256uw1474184267",
        "oss_username": "commitity",
        "oss_password": "commitity",
        "oss_cart": "/usr/local/share/ca-certificates/hub.goodrain.com.crt"
    }
}
}
EOF

}

function install_chaos(){
    log.info "setup chaos"

    chaos_write_cfg

    compose::config_update << EOF
services:
  rbd-chaos:
    image: $RBD_CHAOS
    container_name: rbd-chaos
    command: --log-level=debug --mysql="$MYSQL_USER:$MYSQL_PASSWD@tcp(${MYSQL_HOST:-127.0.0.1}:${MYSQL_PORT:-3306})/$MYSQL_DB"
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    volumes:
    - /logs:/logs
    - /grdata:/grdata
    - /cache:/cache
    - /var/run:/var/run
    - /root/.docker/config.json:/root/.docker/config.json
    - /etc/goodrain/ssh:/home/rain/.ssh
    - /etc/goodrain/etc/chaos/config.json:/run/plugins/config.json
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d
}

function lb_add_forward() {
cat <<EOF > /etc/goodrain/openresty/servers/http/forward.conf
upstream goodrain {
   server 172.30.42.1:8688 max_fails=3 fail_timeout=1s;
   keepalive 10;
}

server {
   listen 80;
   server_name *.goodrain.me goodrain.me;

   location / {
      proxy_pass http://goodrain;
      proxy_set_header Host \$host;
      proxy_redirect off;
      proxy_set_header X-Real-IP \$remote_addr;
      proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
      proxy_connect_timeout 60;
   }
}
EOF

}

function install_lb() {
    log.info "setup lb"

    compose::config_update << EOF
services:
  rbd-lb:
    image: $RBD_LB
    container_name: rbd-lb
    environment:
      NGINX_INIT_PORT: 80
      MYSQL_HOST: ${MYSQL_HOST:-127.0.0.1}
      MYSQL_PORT: ${MYSQL_PORT:-3306}
      MYSQL_USERNAME: $MYSQL_USER
      MYSQL_PASSWORD: $MYSQL_PASSWD
      MYSQL_DATABASE: $MYSQL_DB
      HTTP_SUFFIX_URL: ${EX_DOMAIN#.*}
    volumes:
      - /etc/goodrain/openresty:/usr/local/openresty/nginx/conf
      - /data/openrestry/logs:/usr/local/openresty/nginx/logs
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

    dc-compose up -d

    lb_add_forward

    dc-compose stop rbd-lb
    dc-compose up -d rbd-lb
}

function install_eventlog() {
    log.info "setup eventlog"

    compose::config_update << EOF
services:
  rbd-eventlog:
    image: $RBD_EVENTLOG
    container_name: rbd-eventlog
    environment:
      MYSQL_HOST: ${MYSQL_HOST:-127.0.0.1}
      MYSQL_PORT: ${MYSQL_PORT:-3306}
      MYSQL_USER: $MYSQL_USER
      MYSQL_PASSWORD: $MYSQL_PASSWD
      MYSQL_DATABASE: $MYSQL_DB
      K8S_MASTER: http://127.0.0.1:8181
      CLUSTER_BIND_IP: $HOST_IP
      #- CONSOLE_TOKEN=''
    volumes:
      - /var/log/event-log/:/var/log
      - /etc/goodrain/:/etc/goodrain/
      - /grdata/downloads/log:/grdata/logs
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

function install_mq() {
    log.info "setup mq"

    compose::config_update << EOF
services:
  rbd-mq:
    image: $RBD_MQ
    container_name: rbd-mq
    command: --log-level=debug
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

function write_slogger_config() {
    cat <<EOF > /etc/goodrain/labor.py
# -*- coding: utf8 -*-

DEFAULT_HANDLERS = ['zmq_handler']

beanstalk = {
    "default": {
        "host": "127.0.0.1",
        "port": 11300,
    }
}

region = {
    "url": "http://region.goodrain.me:8888",
}

userconsole = {
    "url": "http://console.goodrain.me",
}

k8s = {
    'api': {
        "url": "http://127.0.0.1:8181/api/v1"
    }
}

etcd = {
    'lock': {
        "host": "127.0.0.1",
        "port": 2379
    }
}

mysql = {
    'region_rw': {
        "host": '$MYSQL_HOST', "port": $MYSQL_PORT,
        "user": "$MYSQL_USER", "passwd": "$MYSQL_PASSWD",
        "db": "region", "charset": "utf8"
    },
    'region_ro': {
        "host": '$MYSQL_HOST', "port": $MYSQL_PORT,
        "user": "$MYSQL_USER", "passwd": "$MYSQL_PASSWD",
        "db": "region", "charset": "utf8"
    },
}

zmq = {
    'service_pub': {
        'address': 'tcp://127.0.0.1:9341'
    },
    'service_sub': {
        'address': [
            'tcp://127.0.0.1:9342'
        ],
        'storage': '/logs'
    },
    'cep_sub': {
        'address': 'tcp://127.0.0.1:9442',
        'storage': '/data/logs/tree-zxtm',
        'topic': 'cep.weblog'
    }
}

opentsdb = {
    'default': {
        'host': '$HOST_IP', 'port': 4242,
    },
}

oss = {
    'ali_shanghai': {
        'id': 'id',
        'secret': 'secret',
        'endpoint': 'oss-cn-shanghai.aliyuncs.com',
    }
}

moudels = {
    "service_container_monitor": {
        "hibernate_consistence": False,
        "container_statics": False,
        "service_running_statics": False,
        "service_event_statics": False
    }
}

# config deploy slug

CLOUD_ASSISTANT = "256uw1474184267"
publish = {
    'slug': {
        # 文件存储路径
        'slug_path': '/grdata/build/tenant/',
        # 数据中心slug存储路径
        'curr_region_path': '/grdata/build/tenant/',
        'curr_region_dir': 'app_publish/',
        # 区域中心slug的ftp配置
        'all_region_ftp': False,
        'all_region_ftp_host': '139.196.88.57',
        'all_region_ftp_port': '10021',
        'all_region_username': 'commitity',
        'all_region_password': 'commitity',
        'all_region_namespace': 'app-publish/',
        # cloud market存储配置OSS
        'oss_ftp': True,
        'oss_ftp_host': '139.196.88.57',
        'oss_ftp_port': '10021',
        'oss_username': 'commitity',
        'oss_password': 'commitity',
        'oss_namespace': 'app-publish/',
    },
    'image': {
        # 当前数据中心镜像仓库
        'curr_registry': 'goodrain.me',
        # cloud assistant镜像仓库
        'all_region_image': False,
        'all_registry': 'oss.goodrain.me',
        # cloud market 镜像仓库
        'oss_image': True,
        'oss_host': 'hub.goodrain.com',
        'oss_namespace': '256uw1474184267',
        'oss_username': 'commitity',
        'oss_password': 'commitity',
        'oss_cart': '/usr/local/share/ca-certificates/hub.goodrain.com.crt',
    }
}

# nginx的负载均衡

MULTI_LB = {
    "NGINX": {
        'enabled': True,
        'http': [
            'http://$HOST_IP:10002',
        ],
        'stream': [
            'http://$HOST_IP:10002',
        ],
    },
    "ZEUX": {
        'enabled': False,
    }
}
EOF
}

function install_slogger() {
    log.info "setup slogger"
    [ -d "/etc/goodrain/ssh" ] || ( mkdir /etc/goodrain/ssh) && (
        chown rain.rain /etc/goodrain/ssh
    )
    [ -d "/grdata/logs" ] || (
        mkdir /grdata/logs && chown rain.rain /grdata/logs
    )
    if [ ! -L "/data/docker_logs" ];then
        mkdir -p /data/service_logs && chown rain.rain /data/docker_logs
    fi
    [ -d "/grdata/build/tenant/" ] || (
        mkdir -p /grdata/build/tenant && chown rain.rain /grdata/build/tenant
    )
    [ -d "/cache/build" ] && (
        chown rain.rain /cache
        chown rain.rain /cache/build
    ) || (
        mkdir -p /cache/build && chown -R rain.rain /cache/
    )
    [ -d "/grdata/cache" ] && (
        chown  rain.rain /grdata/cache
    ) || (
        mkdir -p /grdata/cache
        chown  rain.rain /grdata/cache
    )
    [ -L "/logs" ] || (
        mkdir -p /data/service_logs && chown rain.rain /data/service_logs
        rm -rf /logs
        ln -s /data/service_logs /logs
    )

    write_slogger_config

    compose::config_update << EOF
services:
  rbd-slogger:
    image: $RBD_SLOGGER
    command: basic_group
    container_name: rbd-slogger
    environment:
      REGION_TAG: $REGION_TAG
    volumes:
      - /etc/goodrain/labor.py:/app/labor/etc/regions/$REGION_TAG.py
      - /logs:/logs
      - /grdata:/grdata
      - /data/docker_logs:/data/docker_logs
      - /cache:/cache
      - /var/run:/var/run
      - /root/.docker/config.json:/root/.docker/config.json
      - /etc/goodrain/ssh:/home/rain/.ssh
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "3"
    network_mode: "host"
    restart: always
EOF

}

function run() {
    
    log.info "setup plugins"

    image::done $RBD_API
    image::done $RBD_WORKER
    image::done $RBD_CHAOS
    
    image::done $RBD_LB
    image::done $RBD_EVENTLOG
    image::done $RBD_MQ
    image::done $RBD_WEB

    image::done $RBD_SLOGGER


    install_eventlog
    
    install_api
    install_worker
    install_chaos

    install_lb

    
    install_mq
    install_app_ui
    install_slogger
    dc-compose up -d 
    log.stdout '{
            "status":[ 
            { 
                "name":"install_acp_plugins", 
                "condition_type":"INSTALL_ACP_PLUGINS", 
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