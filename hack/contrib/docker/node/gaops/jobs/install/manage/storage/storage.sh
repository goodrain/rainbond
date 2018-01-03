#!/bin/bash
set -o errexit
set -o pipefail 

OS_VERSION=$1
STORAGE_MODE=${2:-nfs} # 默认nfs，支持custom
NFS_HOST=$3 # nfs_server ip或者domain 
NFS_ENDPOINT=$4 # NFS_server share
NFS_ARGS=$5 #预留参数，当前版本不用

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

NFS_DEST="/grdata"

if [ -z "$NFS_ARGS" ];then
    if [ "$STORAGE_MODE" == "nfs" ];then
        NFS_ARGS="nfs rw 0 0"
    elif [ "$STORAGE_MODE" == "nas" ];then
        NFS_ARGS="nfs4 auto 0 0"
    fi
fi

function package::match(){
    str=$1
    egrep_compile=$2
    echo $str | egrep "$egrep_compile" >/dev/null
}

function package::is_installed() {
    log.info "check package install"
    pkgname=$1
    if [[ $OS_VERSION =~ "7" ]];then
        rpm -qi $pkgname >/dev/null
        if [ $? -gt 0 ];then
            log.info "package $pkgname is not installed"
            return 1
        else
            log.info "package $pkgname is already installed"
            return 0
        fi
    else
        pkginfo=$(apt search $pkgname 2>/dev/null | egrep "^$pkgname/")
        [ -z "pkginfo" ] && {
            log.info "can not find package:$pkgname"
            return 1
        }
        pkgitems=($pkginfo)
        pkglen=${#pkgitems[@]}
        [ $pkglen -lt 4 ] && {
            return 1
        }
        full_version=${pkgitems[1]}
        install_status=${pkgitems[3]}
        package::match $install_status "installed|upgradable" && {
            log.info "$pkgname is already installed"
            return 0
        } || {
            log.info "$pkgname is not installed"
            return 1
        }
    fi
}

function package::install() {
    pkgname=$1
    pkg_version=${2:-*}

    log.info "install $pkgname"

    if [[ $OS_VERSION =~ "7" ]];then
        yum install -y $pkgname
    else
        DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes -o Dpkg::Options::="--force-confold"  $package="$pkg_version"
    fi
}

function sys::path_mounted() {
    dest_dir=$1
    if [ ! -d "$dest_dir" ];then
        log.info "dir $dest_dir not exist"
        return 1
    fi
    df -h | grep $dest_dir >/dev/null && (
        log.info "$dest_dir already mounted"
        return 0
    ) || (
        log.info "$dest_dir not mounted"
        return 1 
    )
}

function check_automount() {
    DEST_ENDPOINT=$(grep $NFS_DEST /etc/fstab | awk '{print $2}')
    if [ "$DEST_ENDPOINT" == "NFS_DEST" ];then
        return 0
    else
        log.info "automount need to set"
        return 1
    fi
}

function write_automount() {
    mount_string="$NFS_HOST:$NFS_ENDPOINT $NFS_DEST $NFS_ARGS"
    echo "$mount_string" >> /etc/fstab
    echo "$mount_stringn write into /etc/fstab"
}

function prepare() {
    log.info "RBD: mount nfs"
    log.info "prepare NFS"
}

function run_manage() {
    
    if [[ $OS_VERSION =~ '7'  ]];then
        package::is_installed nfs-utils $OS_TYPE || (
        log.info "nfs-utils not installed"
        package::install nfs-utils $OS_TYPE || (
            log.error "install failed"
            exit 1
        )
    )
    else
        package::is_installed nfs-common $OS_TYPE || (
        log.info "nfs-common not installed"
        package::install nfs-common $OS_TYPE || (
            log.info "install failed"
            exit 1
        )
    )
    fi
    

    sys::path_mounted $NFS_DEST || (
        mkdir -pv $NFS_DEST
        mount -t nfs $NFS_HOST:$NFS_ENDPOINT $NFS_DEST
    )
    check_automount || write_automount
}

function check_config() {
    
    [ -d "$NFS_DEST" ] || mkdir $NFS_DEST
    grep $NFS_DEST /etc/exports >/dev/null
    if [ $? -eq 0 ];then
        log.info "find required share dir: $NFS_DEST"
        return 0
    else
        log.error "expect $NFS_DEST in /etc/exports, but not found"
        return 1
    fi
}


function write_config() {
    
    echo "$NFS_DEST *(rw,sync,no_root_squash,no_subtree_check)" >> /etc/exports
    log.info "write share config into /etc/exports"
}

function run_server() {
    log.info "setup nfs-server"
    package::is_installed nfs-utils $OS_TYPE || (
        package::install nfs-utils $OS_TYPE || (
            log.error "install faild"
            log.stdout '{
            "status":[ 
            { 
                "name":"install_storage_nfs_server", 
                "condition_type":"INSTALL_STORAGE_NFS_SERVER", 
                "condition_status":"False"
            } 
            ], 
            "exec_status":"Failure",
            "type":"install"
            }'
            exit 1
        )
    )

    IP=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')

    systemctl is-active firewalld.service && (
        firewall-cmd --permanent --zone=public --add-service=nfs
        firewall-cmd --reload
    )

    check_config || write_config

    UNIT=nfs-server.service
    systemctl is-enabled $UNIT || systemctl enable $UNIT
    systemctl is-active $UNIT || systemctl start $UNIT && (
        
        showmount -e 127.0.0.1 2>/dev/null | grep $NFS_DEST
        if [ $? -ne 0 ];then
            systemctl restart $UNIT
        fi
    )


    _EXIT=1
    for (( i = 1; i <= 3; i++ )); do
        sleep 1
        systemctl is-active $UNIT && export _EXIT=0 && break
    done

    if [ $_EXIT -eq 0 ];then
        log.info "NFS INSTALL SUCCESSFUL..."
    else
        log.error "check failed. abort..."
        log.stdout '{
            "status":[ 
            { 
                "name":"start_storage_nfs_server", 
                "condition_type":"START_STORAGE_NFS_SERVER", 
                "condition_status":"False"
            } 
            ], 
            "exec_status":"Failure",
            "type":"install"
            }'
        exit $_EXIT
    fi

}

function run() {
    if [ -z "$NFS_HOST" ];then
        log.info "install nfs server"
        run_server
        log.stdout '{ 
            "global":{
              "STORAGE_MODE":"'$STORAGE_MODE'",
              "NFS_SERVERS":"'$IP'",
              "NFS_ENDPOINT":"'$NFS_DEST'"
            },
            "status":[ 
            { 
                "name":"install_storage_manage_server", 
                "condition_type":"INSTALL_STORAGE_MANAGE_SERVER", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
    else
        log.info "install nfs client"
        run_manage
        log.stdout '{
            "status":[ 
            { 
                "name":"install_storage_manage_client", 
                "condition_type":"INSTALL_STORAGE_MANAGE_CLIENT", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
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