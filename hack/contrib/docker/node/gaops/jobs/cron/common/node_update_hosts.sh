#!/bin/bash

DO_UID=$1
DO_IP=$2
DO_TYPE=$3

function add_uid(){
    grep "$DO_UID" /etc/hosts > /dev/null
    if [ $? -ne 0 ];then
        grep "$DO_IP" /etc/hosts > /dev/null
        if [ $? -ne 0 ];then
            echo "$DO_IP $DO_UID" >> /etc/hosts
        else
            sed -i "s/$DO_IP/$DO_IP $DO_UID/g" /etc/hosts
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
        else
            sed -i "s/$DO_UID//g" /etc/hosts
        fi
    fi
    
}

function run(){
    if [ $DO_TYPE = 'add' ];then
        add_uid
    else
        del_uid
    fi
}

case $1 in
    *)
    run
    ;;
esac