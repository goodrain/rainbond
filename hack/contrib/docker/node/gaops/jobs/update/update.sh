#!/bin/bash

function log.info() {
  echo "       $*"
}

function update_repo(){
    log.info "update repo 3.4.1 to 3.4.2"
    sed -i "s#3.4.1#3.4.2#g" /etc/yum.repos.d/acp.repo
    yum clean all
    yum makecache
}

function update_rbd_version(){
    [ -f "/etc/goodrain/envs/rbd.json" ] && mv /etc/goodrain/envs/rbd.json /etc/goodrain/envs/rbd.json_old
    log.info "wget rbd.json from goodrain.mirrors"
    wget http://repo.goodrain.com/release/3.4.2/gaops/jobs/install/prepare/rbd.json -O /etc/goodrain/envs/rbd.json
    if [ -f " /etc/goodrain/envs/rbd.json" ];then
        curl  http://repo.goodrain.com/release/3.4.2/gaops/jobs/install/prepare/rbd.json -o  /etc/goodrain/envs/rbd.json
    fi
}

function reload_node(){
    log.info "install new node & grctl"
    yum install gr-rainbond-node gr-rainbond-grctl -y
    log.info "update tasks"
    wget http://repo.goodrain.com/release/3.4.2/gaops/jobs/update/update.json -O /usr/share/gr-rainbond-node/gaops/tasks/update_342.json
    systemctl restart rainbond-node
}

function exec_update(){
    uuid=$(grctl node list | grep "manage,compute" | awk '{print $2}')
    log.info "exec tasks redo_rbd_images "
    grctl tasks exec redo_rbd_images -n $uuid
}

function exec_sql(){
    log.info "pass"
}

function run(){
    update_repo
    update_rbd_version
    reload_node
    exec_update
}

case $1 in
    *)
    run
    ;;
esac