#!/bin/bash

DO_UID=$1
DO_IP=$2
DO_TYPE=$3

[ -d "/var/log/rbdnode" ] || mkdir -p /var/log/rbdnode

function add_uid(){
    grep "$DO_UID" /etc/hosts > /dev/null
    if [ $? -ne 0 ];then
        echo "$(date +'%F %X') $(hostname -s): not found $DO_UID" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        grep "$DO_IP" /etc/hosts > /dev/null
        if [ $? -ne 0 ];then
            echo "$(date +'%F %X') $(hostname -s): not found $DO_IP" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
            echo "$DO_IP $DO_UID" >> /etc/hosts
            echo "$(date +'%F %X') $(hostname -s): add $DO_IP $DO_UID" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        else
            sed -i "s/$DO_IP/$DO_IP $DO_UID/g" /etc/hosts
            echo "$(date +'%F %X') $(hostname -s): update $DO_IP $DO_UID" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        fi
    else
        grep "$DO_UID" /etc/hosts | grep "$DO_IP" > /dev/null
        if [ $? -eq 0 ];then
            echo "$(date +'%F %X') $(hostname -s): not update" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        else
            echo "$(date +'%F %X') $(hostname -s): error found $OD_UID ,not found $DO_IP" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        fi
    fi
}

function del_uid(){

    old_md5=$(cat /etc/hosts | grep "^${DO_IP}" | sed "s#${DO_UID}##g" | sed 's/ //g' | md5sum | awk '{print $1}')
    new_md5=$(echo "${DO_IP} ${DO_UID} " |  sed "s#${DO_UID}##g" | sed 's/ //g' | md5sum | awk '{print $1}')
    grep "$DO_UID" /etc/hosts > /dev/null
    if [ $? -eq 0 ];then
        if [ $new_md5 = $old_md5 ];then
            sed -i "s/$DO_IP $DO_UID//g" /etc/hosts
            echo "$(date +'%F %X') $(hostname -s): del $DO_IP $DO_UID" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        else
            sed -i "s/$DO_UID//g" /etc/hosts
            echo "$(date +'%F %X') $(hostname -s): del $OD_UID" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        fi
    fi
    
}

function run(){
    if [ "$DO_IP" == "$DO_UID" ];then
        echo "$(date +'%F %X') $(hostname -s): not change $DO_IP" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
    else
        if [ $DO_TYPE = 'add' ];then
            echo "$(date +'%F %X') $(hostname -s): add $DO_UID for $DO_IP start" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
            add_uid
            echo "$(date +'%F %X') $(hostname -s): add $DO_UID for $DO_IP end" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        else
            echo "$(date +'%F %X') $(hostname -s): del $DO_UID for $DO_IP start" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
            del_uid
            echo "$(date +'%F %X') $(hostname -s): del $DO_UID for $DO_IP end" >> /var/log/rbdnode/node_update_hosts_$(date +'%Y_%m_%d').log
        fi
    fi
    
}

case $1 in
    *)
    run
    ;;
esac