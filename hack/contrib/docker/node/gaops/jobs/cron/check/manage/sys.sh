#!/bin/bash

#
# 系统信息
#


RELEASE_INFO=$(cat /etc/os-release | grep "^VERSION=" | awk -F '="' '{print $2}' | awk '{print $1}' | cut -b 1-5)
if [[ $RELEASE_INFO =~ "7" ]];then
    OS=$(cat /etc/redhat-release | awk '{print $1,$4}')
elif [[ $RELEASE_INFO =~ "14" ]];then
    OS=$(lsb_release -d | awk '{print $2,$3,$4}')
elif [[ $RELEASE_INFO =~ "16" ]];then
    OS=$(grep "PRETTY_NAME" /etc/os-release  | awk -F '[="]' '{print $3}')
elif [[ $RELEASE_INFO =~ "9" ]];then
    OS=$(grep "PRETTY_NAME" /etc/os-release  | awk -F '[="]' '{print $3}')
else
    OS='null'
fi

KERNEL=$(uname -rv)
PLATFORM=$(uname -i)
LOGIC_CORES=$(cat /proc/cpuinfo |grep -c "processor")
MEMORY=$(free -mth | grep "Mem" | awk '{print $2}')

log.stdout() {
    echo "$*" >&2
}

sysinfo() {
    log.stdout '{
                "OS":"'${OS}'",
                "KERNEL":"'${KERNEL}'",
                "PLATFORM":"'${PLATFORM}'",
                "LOGIC_CORES":"'${LOGIC_CORES}'",
                "MEMORY":"'${MEMORY}'"
        }'
}

case $1 in
    *)
        sysinfo
    ;;
esac