#!/bin/bash

DO_UID=$1
DO_IP=$2
DO_TYPE=$3

function add_uid(){
    grep "$DO_UID" /etc/hosts > /dev/null
    if [ $? -ne 0 ];then
        sed -i "s/$DO_IP/$DO_IP $DO_UID/g" /etc/hosts
    fi
}

function del_uid(){
    grep "$DO_UID" /etc/hosts > /dev/null
    if [ $? -eq 0 ];then
        sed -i "s/$DO_UID//g" /etc/hosts
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