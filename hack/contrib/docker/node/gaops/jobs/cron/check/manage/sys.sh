#!/bin/bash

#
# 系统信息
#

OS=$(lsb_release -d | grep "Description"|awk -F ':' '{print $2}')
KERNEL=$(uname -rv)
PLATFORM=$(uname -i)
LOGIC_CORES=$(cat /proc/cpuinfo |grep -c "processor")
MEMORY=$(dmidecode -t memory |grep "Maximum Capacity"| awk -F: '{print $2}')

log.stdout() {
    echo "$*" >&2
}


just_echo() {
echo '
  {
    "OS":"'${OS}'",
    "KERNEL":"'${KERNEL}'"
    "PLATFORM":"'${PLATFORM}'",
    "LOGIC_CORES":"'${LOGIC_CORES}'",
    "MEMORY":"'${MEMORY}'"
  }
'
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