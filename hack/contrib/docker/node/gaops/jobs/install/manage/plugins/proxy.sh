#!/bin/bash

REPO_VER=$1
RBD_REPO_EXPAND=${2:-0}
LANG_SERVER=${3:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}
MAVEN_SERVER=${4:-$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')}

RBD_IMAGE_PROXY_NAME=$(jq --raw-output '."rbd-proxy".name' /etc/goodrain/envs/rbd.json)
RBD_IMAGE_PROXY_VERSION=$(jq --raw-output '."rbd-proxy".version' /etc/goodrain/envs/rbd.json)
RBD_PROXY=$RBD_IMAGE_PROXY_NAME:$RBD_IMAGE_PROXY_VERSION


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

function add_console_vhost() {
    log.info "add console_vhost"
    cat > /etc/goodrain/proxy/sites/console <<EOF
upstream console {
  server 127.0.0.1:7070;
}

server {
    listen 172.30.42.1:8688;
    server_name console.goodrain.me;

    location / {
        proxy_pass http://console;
        proxy_set_header Host \$host;
        proxy_redirect off;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_connect_timeout 60;
        proxy_read_timeout 600;
        proxy_send_timeout 600;
    }
}
EOF
}
function add_registry_vhost() {
    log.info "add registry vhost"
    cat > /etc/goodrain/proxy/sites/registry <<EOF
upstream registry {
  server 127.0.0.1:5000;
}

server {
    listen       172.30.42.1:443;
    server_name  goodrain.me;

    ssl          on;
    ssl_certificate ssl/goodrain.me/server.crt;
    ssl_certificate_key ssl/goodrain.me/server.key;

    client_max_body_size 0;

    chunked_transfer_encoding on;

    location /v2/ {
      #add_header 'Docker-Distribution-Api-Version' 'registry/2.3.1' always;
        proxy_pass                          http://registry;
        proxy_set_header  Host              \$http_host;   # required for docker client's sake
        proxy_set_header  X-Real-IP         \$remote_addr; # pass on real client's IP
        proxy_set_header  X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header  X-Forwarded-Proto \$scheme;
        proxy_read_timeout                  900;


    }
}

server {
    listen       172.30.42.1:8688;
    server_name  goodrain.me;


    client_max_body_size 0;

    chunked_transfer_encoding on;

    location /v2/ {
      #add_header 'Docker-Distribution-Api-Version' 'registry/2.3.1' always;

        proxy_pass                          http://registry;
        proxy_set_header  Host              \$http_host;   # required for docker client's sake
        proxy_set_header  X-Real-IP         \$remote_addr; # pass on real client's IP
        proxy_set_header  X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header  X-Forwarded-Proto \$scheme;
        proxy_read_timeout                  900;
    }
}
EOF
}
function add_maven_vhost() {
    log.info "add maven vhost"
cat > /etc/goodrain/proxy/sites/maven <<EOF
upstream maven {

  server 127.0.0.1:8081;

  check interval=3000 rise=2 fall=1 timeout=1000 type=http default_down=true;
  check_http_send "HEAD /artifactory/pkg_lang/monitor.html HTTP/1.0\r\n\r\n";
  check_http_expect_alive http_2xx;
  keepalive 10;
}

server {
    listen 172.30.42.1:8688;
    server_name maven.goodrain.me;

    location / {
        rewrite ^/(.*)$ /artifactory/libs-release/\$1 break;
        proxy_pass http://maven;
        proxy_set_header Host \$host;
        proxy_redirect off;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_connect_timeout 60;
        proxy_read_timeout 600;
        proxy_send_timeout 600;
    }

    location /monitor {
        return 204;
    }
}
EOF
}

function add_lang_vhost() {
    log.info "add lang vhost"
    cat > /etc/goodrain/proxy/sites/lang <<EOF
upstream lang {

  server 127.0.0.1:8081;

  check interval=3000 rise=2 fall=1 timeout=1000 type=http default_down=false;
  check_http_send "HEAD /artifactory/pkg_lang/monitor.html HTTP/1.0\r\n\r\n";
  check_http_expect_alive http_2xx;
  keepalive 10;
}

server {
    listen 172.30.42.1:8688;
    server_name lang.goodrain.me;

    rewrite ^/(.*)$ /artifactory/pkg_lang/\$1 break;

    location / {
        proxy_pass http://lang;
        proxy_set_header Host \$host;
        proxy_redirect off;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_connect_timeout 60;
        proxy_read_timeout 600;
        proxy_send_timeout 600;
    }
}
EOF
}

function proxy() {
    log.info "setup proxy"

    image::exist $RBD_PROXY || (
        log.info "pull image: $RBD_PROXY"
        image::pull $RBD_PROXY || (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"docker_pull_proxy", 
                    "condition_type":"DOCKER_PULL_PROXY_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )
    
        compose::config_update << EOF
version: '2.1'
services:
  rbd-proxy:
    image: $RBD_PROXY
    container_name: rbd-proxy
    volumes:
      - /etc/goodrain/proxy/sites:/usr/local/tengine/conf/sites
      - /etc/goodrain/proxy/ssl:/usr/local/tengine/conf/ssl
      - /grdata:/grdata
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

function prepare() {
    mkdir -p /etc/goodrain/proxy/{sites,ssl}
    mkdir -p /etc/goodrain/proxy/ssl/goodrain.me

    cat <<EOF > /etc/goodrain/proxy/ssl/goodrain.me/server.crt
-----BEGIN CERTIFICATE-----
MIIClTCCAf4CCQCrz/TYniQE3zANBgkqhkiG9w0BAQsFADCBjTELMAkGA1UEBhMC
Q04xEDAOBgNVBAgMB0JlaWppbmcxEDAOBgNVBAcMB0JlaWppbmcxETAPBgNVBAoM
CGdvb2RyYWluMQ8wDQYDVQQLDAZzeXN0ZW0xFDASBgNVBAMMC2dvb2RyYWluLm1l
MSAwHgYJKoZIhvcNAQkBFhFyb290QGdvb2RyYWluLmNvbTAgFw0xNjA0MjYxMTE0
NTZaGA8yMTE2MDQwMjExMTQ1NlowgY0xCzAJBgNVBAYTAkNOMRAwDgYDVQQIDAdC
ZWlqaW5nMRAwDgYDVQQHDAdCZWlqaW5nMREwDwYDVQQKDAhnb29kcmFpbjEPMA0G
A1UECwwGc3lzdGVtMRQwEgYDVQQDDAtnb29kcmFpbi5tZTEgMB4GCSqGSIb3DQEJ
ARYRcm9vdEBnb29kcmFpbi5jb20wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGB
ALSWCeeDuge8N9coS2w+7q1M9RdTI5O85E984t97yTJNOVWcxCjPZRkTSEGPXjuv
QUCqBKbXWJX++dcDE8Xrx5yGQZywNOUi4sBjxvkO0+kPH3cBcZYb6+Jt2Boyk0ja
lPPJ1n7YlIfbps+MCGoSlsozh1ms8/MmSdDhYnA2HhZhAgMBAAEwDQYJKoZIhvcN
AQELBQADgYEAcp2ETrYEvzxty5fFQXuEUdJQBjXUUaO4YuFuAHZnX0mBdLFs8JHt
Dv5SVos+Rd/zF9Szg68uBOzkrFODygyzUjPgUtP1oIrPMFgvraYmbBQNdzT/7zBN
OIBrj5fMeg27zqsV/2Qr1YuzfMZcgQG9KtPSe57RZH9kF7pCl+cqetc=
-----END CERTIFICATE-----
EOF

cat <<EOF > /etc/goodrain/proxy/ssl/goodrain.me/server.key
-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQC0lgnng7oHvDfXKEtsPu6tTPUXUyOTvORPfOLfe8kyTTlVnMQo
z2UZE0hBj147r0FAqgSm11iV/vnXAxPF68echkGcsDTlIuLAY8b5DtPpDx93AXGW
G+vibdgaMpNI2pTzydZ+2JSH26bPjAhqEpbKM4dZrPPzJknQ4WJwNh4WYQIDAQAB
AoGBAIe0bJL28XhYn9nm5O7eR/whZdj2WDjwbN2y6saoviQ31gsY+Gv2lnGGhPkH
ZPgTFkUivsYl8+McLeG+5UAJlAEvLqA9YchdVN6cAzhZfBtluMQgDfULKQVGRrrQ
qCxjW3ZkTZqhC5ZDbF7oDWLdvHE0W4wqze7DrzFYDIf+ZOFFAkEA5TGxAK7IBklV
5Jae+yWQi+WiLTCEng7LhXpOOJQz++W0DP+yxru2oFgLs+93o4TiV2gMm9uNxXVJ
6DFVe3oMbwJBAMm0+RIVn50uzxnacP8pb9EDpb2duoxMYMVSKbtysmZkOKS8UDTk
KGyTsens4iMj2t7ziSTw5z0RUY6F096KEi8CQQDQO2h8nU/AXmq6Z5qDxaphYD4L
XpRu4jRIzkk5IHVmfFksojg0VSHk5nmjfoMtPrNCBJfIFx7kct62JfRrXgTjAkAT
cTM00ArDjth9iHWt0qOphO172nE5xr7pJiNJoyOZBP4EuvYMMxXGaXITtzaQ5orZ
RKYqfmH7m+i9kR6765kXAkAFLj7zUCgfwuCrZDEhxUDjzRZuO/UJC0jN8xoy5cMg
MNFZqbv4qs89Mf6AuYaDCD+NrpMHJeCeeUpUeboe6yQg
-----END RSA PRIVATE KEY-----
EOF
}

function run() {
    
    log.info "setup rbd proxy"
    add_console_vhost
    
    add_registry_vhost
    
    add_lang_vhost
    add_maven_vhost

    if [ $RBD_REPO_EXPAND -ne 0 ];then
        sed -i "s#127.0.0.1#$MAVEN_SERVER#g" /etc/goodrain/proxy/sites/maven
        sed -i "s#127.0.0.1#$LANG_SERVER#g" /etc/goodrain/proxy/sites/lang
    fi 

    proxy

    _EXIT=1
    for ((i=1;i<=3;i++ )); do
        sleep 3
        log.info "retry $i get rbd-proxy "
        dc-compose ps | grep "proxy" && export _EXIT=0 && break
    done
    
    if [ $_EXIT -eq 0 ];then
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_plugins", 
                    "condition_type":"INSTALL_PLUGINS", 
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
                "name":"install_plugins", 
                "condition_type":"INSTALL_PLUGINS", 
                "condition_status":"False"
            } 
            ], 
            "exec_status":"Failure",
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