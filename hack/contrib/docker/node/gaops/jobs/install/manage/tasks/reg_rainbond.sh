#!/bin/bash


#
# 向云帮注册
#

domain=$(cat /data/.domain.log | awk '{print $1}')
uuid=$(cat /etc/goodrain/host_uuid.conf | awk -F '=' '{print $2}')
if [ -f "/etc/goodrain/envs/.exip" ];then
    ex_ip=$(cat /etc/goodrain/envs/.exip | awk '{print $1}')
else
    ex_ip=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')
fi
if [ ! -z $domain ];then
    curl --connect-timeout 20 http://reg.rbd.goodrain.org/reg?domain=$domain\&uuid=$uuid\&ex_ip=$ex_ip
fi
