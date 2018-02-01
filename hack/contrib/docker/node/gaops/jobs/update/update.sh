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
    wget http://repo.goodrain.com/release/3.4.2/gaops/jobs/update/update.json -O /usr/share/gr-rainbond-node/gaops/tasks/update_342_group.json
    systemctl restart rainbond-node
}

function exec_update(){
    uuid=$(grctl node list | grep "manage,compute" | awk '{print $2}')
    log.info "exec tasks redo_rbd_images <nodeid:$uuid>"
    grctl tasks exec redo_rbd_images -n $uuid
}

function exec_sql(){
    log.info "pass"
    docker exec 

    docker exec rbd-db mysql -e "use console;ALTER TABLE tenant_service ADD update_time DATETIME DEFAULT NOW();ALTER TABLE tenant_service_delete ADD update_time DATETIME DEFAULT NOW();ALTER TABLE tenant_service ADD tenant_service_group_id INT(11) NULL DEFAULT 0;"
    
    docker exec rbd-db mysql -e "use console;ALTER TABLE app_service_group ADD deploy_time datetime DEFAULT NULL;
ALTER TABLE app_service_group ADD installed_count INT(11) NULL DEFAULT 0;
ALTER TABLE app_service_group ADD source VARCHAR(32) NULL DEFAULT 'local';
ALTER TABLE app_service_group ADD enterprise_id INT(11) NULL DEFAULT 0;
ALTER TABLE app_service_group ADD share_scope VARCHAR(20) NULL DEFAULT '';
ALTER TABLE app_service_group ADD is_publish_to_market tinyint(1) DEFAULT 0;
CREATE TABLE tenant_service_group (
  ID int(11) NOT NULL AUTO_INCREMENT,
  tenant_id varchar(32) DEFAULT NULL,
  group_name varchar(64) DEFAULT NULL,
  group_alias varchar(64) DEFAULT NULL,
  group_key varchar(32) DEFAULT NULL,
  group_version varchar(32) DEFAULT NULL,
  region_name varchar(20) DEFAULT NULL,
  service_group_id int(11) DEFAULT '0',
  PRIMARY KEY (ID)
) ENGINE=InnoDB AUTO_INCREMENT=38 DEFAULT CHARSET=utf8;
ALTER TABLE app_service ADD update_version INT(11) NULL DEFAULT 1;
ALTER TABLE app_service_volume ADD volume_type VARCHAR(30) NULL DEFAULT '';
ALTER TABLE app_service_volume ADD volume_name VARCHAR(100) NULL DEFAULT '';
ALTER TABLE tenant_service_delete ADD tenant_service_group_id INT(11) NULL DEFAULT 0;



"
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