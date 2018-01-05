#!/bin/bash
set -o errexit
set -o pipefail 

OS_VERSION=$1
STORAGE_MODE=$2 # 默认nfs，支持custom
NFS_HOST=$3 # nfs_server ip或者domain 
NFS_ENDPOINT=$4 # NFS_server share
NFS_DEST=${5:-"/grdata"}
NFS_ARGS=$6 #预留参数，当前版本不用

# NFS_HOST:NFS_ENDPOINT  NFS_DEST NFS_ARGS
# 10.0.2.11,10.0.2.12,10.0.2.13:/ /grdata ceph    name=admin,secret=AQC5CQpW+85zLhAAd2mR+fjvVNZLqheCL+zqzg== 0 2
#NFS_DEST="/grdata"

if [ -z "$NFS_ARGS" ];then
    if [ "$STORAGE_MODE" == "nfs" ];then
        NFS_ARGS="nfs rw 0 0"
    elif [ "$STORAGE_MODE" == "nas" ];then
        NFS_ARGS="nfs4 auto 0 0"
    fi
fi

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


function package::match(){
    str=$1
    egrep_compile=$2
    echo $str | egrep "$egrep_compile" >/dev/null
}

function package::is_installed() {
    echo "check package install"
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

function run() {
    log.info "RBD: configure storage"
    if [[ "$OS_VERSION" =~ '7'  ]];then
        package::is_installed nfs-utils || (
        log.info "nfs-utils not installed"
        package::install nfs-utils || (
            log.error "install failed"
            log.stdout '{
                    "status":[ 
                    { 
                        "name":"install_storage_nfs_client", 
                        "condition_type":"INSTALL_STORAGE_NFS_CLIENT", 
                        "condition_status":"False"
                    } 
                    ], 
                    "exec_status":"Failure",
                    "type":"install"
                    }'
            exit 1
        )
    )
    else
        package::is_installed nfs-common || (
        log.info "nfs-common not installed"
        package::install nfs-common|| (
            log.error "install failed"
            log.stdout '{
                    "status":[ 
                    { 
                        "name":"install_storage_nfs_client", 
                        "condition_type":"INSTALL_STORAGE_NFS_CLIENT", 
                        "condition_status":"False"
                    } 
                    ], 
                    "exec_status":"Failure",
                    "type":"install"
                    }'
            exit 1
        )
    )
    fi
    

    sys::path_mounted $NFS_DEST || (
        mkdir -pv $NFS_DEST
        mount -t nfs -o vers=4.0  $NFS_HOST:$NFS_ENDPOINT $NFS_DEST
    )
    check_automount || write_automount
    log.stdout '{
            "status":[ 
            { 
                "name":"install_storage_client", 
                "condition_type":"INSTALL_STORAGE_CLIENT", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

case $1 in
    * )
        run
        ;;
esac