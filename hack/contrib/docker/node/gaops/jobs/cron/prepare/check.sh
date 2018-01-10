#!/bin/bash
#
# Run me with:
#
#
#
# This script will:
#
# 1. check system info
#
# 2. check services like docker & etcd & k8s
#
# 3. check ports

which_cmd() {
    which "${1}" 2>/dev/null || \
        command -v "${1}" 2>/dev/null
}

check_cmd() {
    which_cmd "${1}" >/dev/null 2>&1 && return 0
    return 1
}

setup_terminal() {
    TPUT_RESET=""
    TPUT_BLACK=""
    TPUT_RED=""
    TPUT_GREEN=""
    TPUT_YELLOW=""
    TPUT_BLUE=""
    TPUT_PURPLE=""
    TPUT_CYAN=""
    TPUT_WHITE=""
    TPUT_BGBLACK=""
    TPUT_BGRED=""
    TPUT_BGGREEN=""
    TPUT_BGYELLOW=""
    TPUT_BGBLUE=""
    TPUT_BGPURPLE=""
    TPUT_BGCYAN=""
    TPUT_BGWHITE=""
    TPUT_BOLD=""
    TPUT_DIM=""
    TPUT_UNDERLINED=""
    TPUT_BLINK=""
    TPUT_INVERTED=""
    TPUT_STANDOUT=""
    TPUT_BELL=""
    TPUT_CLEAR=""

    # Is stderr on the terminal? If not, then fail
    test -t 2 || return 1

    if check_cmd tput
    then
        if [ $(( $(tput colors 2>/dev/null) )) -ge 8 ]
        then
            # Enable colors
            TPUT_RESET="$(tput sgr 0)"
            TPUT_BLACK="$(tput setaf 0)"
            TPUT_RED="$(tput setaf 1)"
            TPUT_GREEN="$(tput setaf 2)"
            TPUT_YELLOW="$(tput setaf 3)"
            TPUT_BLUE="$(tput setaf 4)"
            TPUT_PURPLE="$(tput setaf 5)"
            TPUT_CYAN="$(tput setaf 6)"
            TPUT_WHITE="$(tput setaf 7)"
            TPUT_BGBLACK="$(tput setab 0)"
            TPUT_BGRED="$(tput setab 1)"
            TPUT_BGGREEN="$(tput setab 2)"
            TPUT_BGYELLOW="$(tput setab 3)"
            TPUT_BGBLUE="$(tput setab 4)"
            TPUT_BGPURPLE="$(tput setab 5)"
            TPUT_BGCYAN="$(tput setab 6)"
            TPUT_BGWHITE="$(tput setab 7)"
            TPUT_BOLD="$(tput bold)"
            TPUT_DIM="$(tput dim)"
            TPUT_UNDERLINED="$(tput smul)"
            TPUT_BLINK="$(tput blink)"
            TPUT_INVERTED="$(tput rev)"
            TPUT_STANDOUT="$(tput smso)"
            TPUT_BELL="$(tput bel)"
            TPUT_CLEAR="$(tput clear)"
        fi
    fi

    return 0
}
setup_terminal || echo >/dev/null

progress() {
    echo >&2 " --- ${TPUT_DIM}${TPUT_BOLD}${*}${TPUT_RESET} --- "
}

check_ok() {
    printf >&2 "${TPUT_BGGREEN}${TPUT_WHITE}${TPUT_BOLD} OK ${TPUT_RESET} ${*} \n\n"
}

check_info() {
    printf >&2 "${TPUT_BGGREEN}${TPUT_WHITE}${TPUT_BOLD} INFO ${TPUT_RESET} ${*} \n\n"
}

check_failed() {
    printf >&2 "${TPUT_BGRED}${TPUT_WHITE}${TPUT_BOLD} FAILED ${TPUT_RESET} ${*} \n\n"
}

fatal() {
    printf >&2 "${TPUT_BGRED}${TPUT_WHITE}${TPUT_BOLD} ABORTED ${TPUT_RESET} ${*} \n\n"
    echo start_status=1 > /tmp/start_status
    exit 1
}

banner() {

local l1=" ^" \
      l2=" |  ____       _       _                     _                               " \
      l3=" | |  _ \ __ _(_)_ __ | |__   ___  _ __   __| |                              " \
      l4=" | | |_) / _\` | | '_ \| '_ \ / _  | '_ \ / _\` |                              " \
      l5=" | |  _ < (_| | | | | | |_) | (_) | | | | (_| |                              " \
      l6=" | |_| \_\__,_|_|_| |_|_.__/ \___/|_| |_|\__,_|                              " \
      l7=" +----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+--->" \
      sp="                                                                             " \
      rbd="Goodrain Rainbond" start end msg="${*}" chartcolor="${TPUT_DIM}"

    [ ${#msg} -lt ${#rbd} ] && msg="${msg}${sp:0:$(( ${#rbd} - ${#msg}))}"
    [ ${#msg} -gt $(( ${#l4} - 20 )) ] && msg="${msg:0:$(( ${#l4} - 23 ))}..."

    start="$(( ${#l4} ))"
    [ $(( start + ${#msg} + 4 )) -gt ${#l4} ] && start=$((${#l4} - ${#msg} - 4))
    end=$(( ${start} + ${#msg} + 4 ))
    echo >&2
    echo >&2 "${chartcolor}${l1}${TPUT_RESET}"
    echo >&2 "${chartcolor}${l2}${TPUT_RESET}"
    echo >&2 "${chartcolor}${l3}${TPUT_RESET}"
    echo >&2 "${chartcolor}${l4:0:start}${sp:0:2}${TPUT_RESET}${TPUT_BOLD}${TPUT_GREEN}${rbd}${TPUT_RESET}${chartcolor}${sp:0:$((end - start - 2 - ${#netdata}))}${l4:end:$((${#l4} - end))}${TPUT_RESET}"
    echo >&2 "${chartcolor}${l5:0:start}${sp:0:2}${TPUT_RESET}${TPUT_BOLD}${TPUT_CYAN}${msg}${TPUT_RESET}${chartcolor}${sp:0:2}${l5:end:$((${#l5} - end))}${TPUT_RESET}"
    echo >&2 "${chartcolor}${l6}${TPUT_RESET}"
    echo >&2 "${chartcolor}${l7}${TPUT_RESET}"
    echo >&2

}

ESCAPED_PRINT_METHOD=
printf "%q " test >/dev/null 2>&1
[ $? -eq 0 ] && ESCAPED_PRINT_METHOD="printfq"
escaped_print() {
    if [ "${ESCAPED_PRINT_METHOD}" = "printfq" ]
    then
        printf "%q " "${@}"
    else
        printf "%s" "${*}"
    fi
    return 0
}

check_logfile="/dev/null"
check() {
    local user="${USER--}" dir="${PWD}" info info_console

    if [ "${UID}" = "0" ]
        then
        info="[root ${dir}]# "
        info_console="[${TPUT_DIM}${dir}${TPUT_RESET}]# "
    else
        info="[${user} ${dir}]$ "
        info_console="[${TPUT_DIM}${dir}${TPUT_RESET}]$ "
    fi

    printf >> "${run_logfile}" "${info}"
    escaped_print >> "${run_logfile}" "${@}"
    printf >> "${run_logfile}" " ... "

    printf >&2 "${info_console}${TPUT_BOLD}${TPUT_YELLOW}"
    escaped_print >&2 "${@}"
    printf >&2 "${TPUT_RESET}\n"

    "${@}"

    local ret=$?
    if [ ${ret} -ne 0 ]
        then
        check_failed
        printf >> "${run_logfile}" "FAILED with exit code ${ret}\n"
    else
        check_ok
        printf >> "${run_logfile}" "OK\n"
    fi

    return ${ret}
}

export PATH="${PATH}:/usr/local/bin:/usr/local/sbin"

curl="$(which_cmd curl)"
wget="$(which_cmd wget)"
bash="$(which_cmd bash)"

if [ -z "${BASH_VERSION}" ]
then
    # we don't run under bash
    if [ ! -z "${bash}" -a -x "${bash}" ]
    then
        BASH_MAJOR_VERSION=$(${bash} -c 'echo "${BASH_VERSINFO[0]}"')
    fi
else
    # we run under bash
    BASH_MAJOR_VERSION="${BASH_VERSINFO[0]}"
fi

HAS_BASH4=1
if [ -z "${BASH_MAJOR_VERSION}" ]
then
    echo >&2 "No BASH is available on this system"
    HAS_BASH4=0
elif [ $((BASH_MAJOR_VERSION)) -lt 4 ]
then
    echo >&2 "No BASH v4+ is available on this system (installed bash is v${BASH_MAJOR_VERSION}"
    HAS_BASH4=0
fi

SYSTEM="$(uname -s)"
OS="$(uname -o)"
MACHINE="$(uname -m)"
OSINFO="$(egrep "(^PRETTY_NAME)" /etc/os-release | awk -F '[="]' '{print $3}')"

function check_os(){
    echo $OSINFO | grep -i "centos" > /dev/null
    if [ $? -eq 0 ];then
        OS_VERSION=$(cat /etc/redhat-release | awk '{print $4}')
    else
        OS_VERSION=$(grep "VERSION=" /etc/os-release  | awk -F '"' '{print $2}')
    fi

    export OS_VERSION=$OS_VERSION
}

function run(){

check_os

CPU_NUM=$(grep "cores"  /proc/cpuinfo | awk '{print $NF}')
MEM_INFO=$(grep "MemTotal"  /proc/meminfo | awk '{print $2}')
NUM=1000
MEM=`expr $MEM_INFO / $NUM / $NUM`
DISK=$(df -h | grep "/$" | awk '{print $2}')

cat <<EOF
System            : ${OS}
Operating System  : ${OSINFO}
Operating System Version : ${OS_VERSION}
Machine           : ${MACHINE}
BASH major version: ${BASH_MAJOR_VERSION}
CPU: ${CPU_NUM}core ===> At least 2cores
MEM: ${MEM}G ===> At least 4G
DISK: ${DISK} ===> At least 40G
NETWORK: "Make sure the IP is static"

EOF
if [ $CPU_NUM -lt 2 ] || [ $MEM -lt 4 ];then
banner "check failed"
fi
check_info "http://www.rainbond.com/docs/stable/getting-started/pre-install.html"
}

case $1 in 
    *)
    run
    ;;
esac