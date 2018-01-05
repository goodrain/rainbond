#!/bin/bash

REPO_VER=${1:-3.4.1}
K8S_IPS=${2} #kubeapi ip
HUB_IPS=${3}


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

function compose::confict() {
    service_name=$1
    compose::config_remove $service_name
    remove_ctn_ids=$(docker ps --filter label=com.docker.compose.service=${service_name} -q)
    if [ -n "$remove_ctn_ids" ];then
        echo "remove containers create by docker-compose for service $service_name "
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

function image::exist() {
    IMAGE=$1
    docker images  | sed 1d | awk '{print $1":"$2}' | grep $IMAGE >/dev/null 2>&1
    if [ $? -eq 0 ];then
        echo "image $IMAGE exists"
        return 0
    else
        echo "image $IMAGE not exists"
        return 1
    fi
}

function image::pull() {
    IMAGE=$1
    docker pull $IMAGE
    if [ $? -eq 0 ];then
        echo "pull image $IMAGE success"
        return 0
    else
        echo "pull image $IMAGE failed"
        return 1
    fi
}

function image::package() {
    ex_pkg=$1
    docker run --rm -v /var/run/docker.sock:/var/run/docker.sock hub.goodrain.com/dc-deploy/archiver $ex_pkg
}

function vhost::exit() {
    vhost=$1
    CONFIG_ROOT="/etc/goodrain/proxy"
    [ -f "$CONFIG_ROOT/sites/$vhost" ] || return 1
}

function vhost::reload() {
    docker exec rbd-proxy nginx -t 2>&1 
    if [ $? -ne 0 ];then
        echo "check tengine config failed"
        return 1
    fi
    echo "restart tengine"
    docker exec rbd-proxy nginx -s reload && sleep 1
}

function vhost::write() {
    vhost=$1
    CONFIG_ROOT="/etc/goodrain/proxy"
    mkdir -p $CONFIG_ROOT/sites $CONFIG_ROOT/ssl
    cat - > $CONFIG_ROOT/sites/$vhost
    return $?
}

function add_kube_vhost() {
    export KUBE_SHARE_DIR="/grdata/services/k8s"
    mkdir -p /etc/goodrain/proxy/ssl/kubeapi.goodrain.me
    log.info "rsync certificate to dir /etc/goodrain/proxy/ssl"
    if [ -d "$KUBE_SHARE_DIR/ssl/" ];then
        rsync -a $KUBE_SHARE_DIR/ssl/ /etc/goodrain/proxy/ssl/kubeapi.goodrain.me
    else
        rsync -a $KUBE_SHARE_DIR/kube-ssl/ /etc/goodrain/proxy/ssl/kubeapi.goodrain.me
    fi
    log.info "create kube for proxy"
    cat <<EOF | vhost::write kube
server {
    listen 172.30.42.1:80;

    root /grdata/kubernetes/;

    index index.html index.htm;

    server_name down.goodrain.me;

    location / {
        try_files $uri $uri/ =404;
    }

    location /monitor {
        return 204;
    }

}

upstream k8sapi {
  
  #server
  
  check interval=3000 rise=2 fall=1 timeout=1000 type=tcp default_down=false;

  consistent_hash \$remote_addr;

  keepalive 1800;
}

server {

  listen 172.30.42.1:6443 ssl;

  server_name         kubeapi.goodrain.me;
  ssl_certificate     ssl/kubeapi.goodrain.me/kubernetes.pem;
  ssl_certificate_key ssl/kubeapi.goodrain.me/kubernetes-key.pem;
  proxy_ssl_certificate ssl/kubeapi.goodrain.me/kube-proxy.pem;
  proxy_ssl_certificate_key     ssl/kubeapi.goodrain.me/kube-proxy-key.pem;
  proxy_ssl_trusted_certificate ssl/kubeapi.goodrain.me/ca.pem;

  location / {
    proxy_pass https://k8sapi;
    proxy_set_header Host \$host;

    # 长连接支持
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering off;

    proxy_redirect off;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_connect_timeout 60;
    proxy_read_timeout 600;
    proxy_send_timeout 600;
 }
}
server {

  listen 127.0.0.1:6443 ssl;

  server_name         kubeapi.goodrain.me;
  ssl_certificate     ssl/kubeapi.goodrain.me/kubernetes.pem;
  ssl_certificate_key ssl/kubeapi.goodrain.me/kubernetes-key.pem;
  proxy_ssl_certificate ssl/kubeapi.goodrain.me/admin.pem;
  proxy_ssl_certificate_key     ssl/kubeapi.goodrain.me/admin-key.pem;
  proxy_ssl_trusted_certificate ssl/kubeapi.goodrain.me/ca.pem;

  location / {
    proxy_pass https://k8sapi;
    proxy_set_header Host \$host;

    # 长连接支持
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering off;

    proxy_redirect off;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_connect_timeout 60;
    proxy_read_timeout 600;
    proxy_send_timeout 600;
 }
}
EOF
  
  log.info "add upstream($(echo $K8S_IPS | tr ',' ' ' | sort -u)) for kube"
  for ii in $(echo $K8S_IPS | tr ',' ' ' | sort -u)
  do
   sed -i "/#server/iserver $ii:6443 max_fails=2 fail_timeout=10s;" /etc/goodrain/proxy/sites/kube
  done

}

function add_registry_vhost() {
    log.info "create registry for proxy"
        cat <<EOF | vhost::write registry
upstream registry {
  
  #server max_fails=2 fail_timeout=10s;
  
  consistent_hash \$remote_addr;
  check interval=3000 rise=2 fall=1 timeout=1000 type=http default_down=false;
  check_http_send "HEAD /v2/ HTTP/1.0\r\n\r\n";
  check_http_expect_alive http_2xx;
  
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

log.info "add upstream($(echo $HUB_IPS | tr ',' ' ' | sort -u)) for registry"
for ii in $(echo $HUB_IPS | tr ',' ' ' | sort -u)
do
    sed -i "/#server/iserver $ii:5000 max_fails=2 fail_timeout=10s;" /etc/goodrain/proxy/sites/registry
done

}

function prepare() {
    log.info "prepare proxy for RBD"
    log.info "check path_mounted /grdata"
    sys::path_mounted /grdata || (
        showmount -e 127.0.0.1 2>&1 | grep "/grdata"
        [ $? -ne 0 ] && (
            log.stdout '{ 
                "status":[ 
                { 
                    "name":"proxy_check_grdata", 
                    "condition_type":"PROXY_CHECK_GRDATA_ERROR", 
                    "condition_status":"False"
                } 
                ], 
                "type":"check"
                }'
            exit 1
        ) || log.info "/grdata ok" 
    )
}


function run() {
    
    
    image::exist $RBD_PROXY || (
        log.info "pull image $RBD_PROXY"
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
    log.info "install dc-compose tool"
    image::package gr-docker-compose
    
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

    dc-compose up -d rbd-proxy
    
    # manage node dont need
    grep "manage" /etc/goodrain/envs/.role >/dev/null
    if [ $? -ne 0 ];then
        
        if [ ! -f "/etc/goodrain/proxy/sites/kube" ];then
            log.info "add kube vhost"
            add_kube_vhost
        else
            dest_md5_k8s=$(echo $K8S_IPS | tr ',' '\n' | sort -u | xargs | md5sum | awk '{print $1}')
            old_md5_k8s=$(grep "^server .*;$"  /etc/goodrain/proxy/sites/kube | tr ':' ' ' | awk '{print $2}' | sort -u | xargs | md5sum | awk '{print $1}')
            if [ $dest_md5_k8s == $old_md5_k8s ];then
                log.info "kube vhost not change"
            else
                log.info "add kube vhost"
                add_kube_vhost
            fi
        fi        
    fi
    
    if [ ! -f "/etc/goodrain/proxy/sites/registry" ];then
        log.info "add registry vhost"
        add_registry_vhost
    else
            dest_md5_hub=$(echo $HUB_IPS | tr ',' '\n' | sort -u | xargs | md5sum | awk '{print $1}')
            old_md5_hub=$(grep "^server .*;$"  /etc/goodrain/proxy/sites/registry | tr ':' ' ' | awk '{print $2}' | sort -u | xargs | md5sum | awk '{print $1}')
            if [ $dest_md5_hub == $old_md5_hub ];then
                log.info "registry vhost not change"
            else
                log.info "add registry vhost"
                add_registry_vhost
            fi
    fi   
    
    vhost::reload
    
    _EXIT=1
    for ((i=1;i<=3;i++ )); do
        sleep 3
        log.info "retry $i time(s) get rbd-proxy "
        dc-compose ps | grep "proxy" && export _EXIT=0 && break
    done

    if [ $_EXIT -eq 0 ];then
        log.info "install plugins for compute node ok"
        log.stdout '{
                "status":[ 
                { 
                    "name":"install_plugins_compute", 
                    "condition_type":"INSTALL_PLUGINS_COMPUTE", 
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
                "name":"install_plugins_compute", 
                "condition_type":"INSTALL_PLUGINS_COMPUTE", 
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
