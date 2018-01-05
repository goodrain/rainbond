#!/bin/bash
set -o errexit
set -o pipefail

OS_VERSION=$1

#[ -z $TERM ] && TERM=xterm-256color

function log.info() {
  echo "       $*"
}

function log.error() {
  echo " !     $*" 
  echo ""
}

function log.stdout() {
    echo "$*" >&2
}


function proc::is_running() {
    proc=$1
    proc_info=$(status $proc 2>&1)
    proc_items=($proc_info)
    status=${proc_items[1]%/*}
    if [ "$status" == "start" ];then
        echo "$proc is running"
        return 0
    else
        echo "$proc is not running: <$proc_info>"
        return 1
    fi
}

function proc::stop() {
    proc=$1
    stop $proc
    return 0
}

function proc::start(){
    proc=$1
    start $proc
    return 0
}

function proc::restart(){
    proc=$1
    if [ "$RELEASE" == "ubuntu/trusty" ];then
        restart $proc
    else
        systemctl restart $proc
    fi
    return 0
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
        yum install -y $pkgname > /dev/stdout 2>&1
    else
        export HOST_IP=$HOST_IP
        export DNS_SERVERS=$DNS
        DEBIAN_FRONTEND=noninteractive apt-get install -y --force-yes -o Dpkg::Options::="--force-confold"  $package="$pkg_version" >/dev/stdout 2>&1
    fi
}

function package::enable() {
    UNIT=$1
    if [[ $OS_VERSION =~ "7" ]];then
        systemctl is-enabled $UNIT || systemctl enable $UNIT
        systemctl is-active $UNIT || systemctl start $UNIT
        _EXIT=1
        for ((i=1;i<=3;i++ )); do
            sleep 1
            systemctl is-active $UNIT || systemctl start $UNIT && export _EXIT=0 && break
        done

        if [ $_EXIT -ne 0 ];then
            log.info "check failed. abort..."
            log.stdout '{
                "status":[ 
                { 
                    "name":"start_'$UNIT'", 
                    "condition_type":"START_'$UNIT'_FAILED", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit $_EXIT
        fi
    else
        UUIT_U=$(echo "$UNIT" | awk -F '.' '{print $1}') 
        proc::is_running $UUIT_U || proc::start $UUIT_U
    fi
}

export KUBE_SHARE_DIR="/grdata/services/k8s"

function prepare() {
    log.info "RBD: install k8s"
    [ -d "/etc/goodrain/envs" ] || mkdir -pv /etc/goodrain/envs
    log.info "prepare k8s"
}

function check_or_create_certificates() {
    log.info "check_or_create_certificates"
    [ -d "$KUBE_SHARE_DIR/ssl" ] || (
        docker run --rm -v $KUBE_SHARE_DIR/ssl:/ssl -w /ssl hub.goodrain.com/dc-deploy/cfssl
    )

    [ -d "/etc/goodrain/kubernetes/ssl" ] || (
        rsync -a $KUBE_SHARE_DIR/ssl /etc/goodrain/kubernetes
    )

    [ -f "/etc/goodrain/kubernetes/token.csv" ] || (
        [ -f "$KUBE_SHARE_DIR/token.csv" ] || (
            export BOOTSTRAP_TOKEN=$(head -c 16 /dev/urandom | od -An -t x | tr -d ' ')
            cat > $KUBE_SHARE_DIR/token.csv <<EOF
${BOOTSTRAP_TOKEN},kubelet-bootstrap,10001,"system:kubelet-bootstrap"
EOF
        )
        cp $KUBE_SHARE_DIR/token.csv /etc/goodrain/kubernetes/token.csv
    )
}

function check_or_create_kubeconfig() {

    log.info "check_or_create_kubeconfig"

    export BOOTSTRAP_TOKEN=$(cut -d ',' -f 1 $KUBE_SHARE_DIR/token.csv)

    [ -f "$KUBE_SHARE_DIR/bootstrap.kubeconfig" ] || (
      kubectl config set-cluster kubernetes \
        --certificate-authority=/etc/goodrain/kubernetes/ssl/ca.pem \
        --embed-certs=true \
        --server=https://127.0.0.1:6443 \
        --kubeconfig=$KUBE_SHARE_DIR/bootstrap.kubeconfig

      kubectl config set-credentials kubelet-bootstrap \
        --token=${BOOTSTRAP_TOKEN} \
        --kubeconfig=$KUBE_SHARE_DIR/bootstrap.kubeconfig

      kubectl config set-context default \
        --cluster=kubernetes \
        --user=kubelet-bootstrap \
        --kubeconfig=$KUBE_SHARE_DIR/bootstrap.kubeconfig

      kubectl config use-context default --kubeconfig=$KUBE_SHARE_DIR/bootstrap.kubeconfig
    )

    [ -f "$KUBE_SHARE_DIR/kube-proxy.kubeconfig" ] || (
      kubectl config set-cluster kubernetes \
        --certificate-authority=/etc/goodrain/kubernetes/ssl/ca.pem \
        --embed-certs=true \
        --server=https://kubeapi.goodrain.me:6443 \
        --kubeconfig=$KUBE_SHARE_DIR/kube-proxy.kubeconfig

      kubectl config set-credentials kube-proxy \
        --client-certificate=/etc/goodrain/kubernetes/ssl/kube-proxy.pem \
        --client-key=/etc/goodrain/kubernetes/ssl/kube-proxy-key.pem \
        --embed-certs=true \
        --kubeconfig=$KUBE_SHARE_DIR/kube-proxy.kubeconfig

      kubectl config set-context default \
        --cluster=kubernetes \
        --user=kube-proxy \
        --kubeconfig=$KUBE_SHARE_DIR/kube-proxy.kubeconfig

      kubectl config use-context default --kubeconfig=$KUBE_SHARE_DIR/kube-proxy.kubeconfig

      mkdir -p /grdata/downloads/k8s
      mkdir -p /grdata/kubernetes
      cp $KUBE_SHARE_DIR/kube-proxy.kubeconfig /grdata/downloads/k8s
      cp $KUBE_SHARE_DIR/kube-proxy.kubeconfig /grdata/kubernetes
      chmod 644 /grdata/downloads/k8s/kube-proxy.kubeconfig
      chmod 644 /grdata/kubernetes/kube-proxy.kubeconfig
    )

    [ -f "$KUBE_SHARE_DIR/kubelet.kubeconfig" ] || (
      kubectl config set-cluster kubernetes \
        --certificate-authority=/etc/goodrain/kubernetes/ssl/ca.pem \
        --embed-certs=true \
        --server=https://kubeapi.goodrain.me:6443 \
        --kubeconfig=$KUBE_SHARE_DIR/kubelet.kubeconfig

      kubectl config set-credentials node \
        --client-certificate=/etc/goodrain/kubernetes/ssl/kubelet.pem \
        --client-key=/etc/goodrain/kubernetes/ssl/kubelet-key.pem \
        --embed-certs=true \
        --kubeconfig=$KUBE_SHARE_DIR/kubelet.kubeconfig

      kubectl config set-context default \
        --cluster=kubernetes \
        --user=node \
        --kubeconfig=$KUBE_SHARE_DIR/kubelet.kubeconfig

      kubectl config use-context default --kubeconfig=$KUBE_SHARE_DIR/kubelet.kubeconfig
    )

    [ -f "$KUBE_SHARE_DIR/admin.kubeconfig" ] || (
      kubectl config set-cluster kubernetes \
        --certificate-authority=/etc/goodrain/kubernetes/ssl/ca.pem \
        --embed-certs=true \
        --server=https://127.0.0.1:6443 \
        --kubeconfig=$KUBE_SHARE_DIR/admin.kubeconfig

      kubectl config set-credentials admin \
        --client-certificate=/etc/goodrain/kubernetes/ssl/admin.pem \
        --client-key=/etc/goodrain/kubernetes/ssl/admin-key.pem \
        --embed-certs=true \
        --kubeconfig=$KUBE_SHARE_DIR/admin.kubeconfig

      kubectl config set-context default \
        --cluster=kubernetes \
        --user=admin \
        --kubeconfig=$KUBE_SHARE_DIR/admin.kubeconfig

      kubectl config use-context default --kubeconfig=$KUBE_SHARE_DIR/admin.kubeconfig
    )

    [ -d "/grdata/kubernetes/" ] && (
        [ -f "/grdata/kubernetes/admin.kubeconfig" ] || cp /grdata/services/k8s/admin.kubeconfig /grdata/kubernetes/
        [ -f "/grdata/kubernetes/kube-proxy.kubeconfig" ] || cp /grdata/services/k8s/kube-proxy.kubeconfig /grdata/kubernetes/
        
    )|| (
        mkdir -pv /grdata/kubernetes/
        cp /grdata/services/k8s/*.kubeconfig /grdata/kubernetes/
    )
}



function install-kube-apiserver() {

    log.info "setup kube-apiserver"
    check_or_create_certificates

    package::is_installed gr-kube-apiserver || (
       package::install gr-kube-apiserver || (
            log.error "install faild"
             log.stdout '{
                "status":[ 
                { 
                    "name":"install_kube-apiserver", 
                    "condition_type":"INSTALL_KUBE-APISERVER", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )

    package::enable kube-apiserver

    check_or_create_kubeconfig

    [ -f "/root/.kube/config" ] || (
        kubectl config set-cluster default-cluster --server=http://127.0.0.1:8181
        kubectl config set-context default --cluster=default-cluster
        kubectl config use-context default
    )
    
    package::enable kube-apiserver

}

function install-kube-controller-manager() {
    log.info "setup kube-controller-manager"
    package::is_installed gr-kube-controller-manager || (
        package::install gr-kube-controller-manager || (
            log.error "install faild"
            log.stdout '{
                "status":[ 
                { 
                    "name":"install_kube-controller-manager", 
                    "condition_type":"INSTALL_KUBE-CONTROLLER-MANAGER", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )
    package::enable kube-controller-manager.service 

}

function install-kube-scheduler() {
    log.info "setup kube-scheduler"
    package::is_installed gr-kube-scheduler || (
        package::install gr-kube-scheduler || (
            log.error "install faild"
            log.stdout '{
                "status":[ 
                { 
                    "name":"install_kube-scheduler", 
                    "condition_type":"INSTALL_KUBE-SCHEDULER", 
                    "condition_status":"False"
                } 
                ], 
                "type":"install"
                }'
            exit 1
        )
    )
    package::enable kube-scheduler.service
}


function run() {
    log.info "install k8s"
    install-kube-scheduler
    install-kube-controller-manager
    install-kube-apiserver
    #[ -f "/etc/goodrain/kubernetes/admin.kubeconfig" ] && rm /etc/goodrain/kubernetes/admin.kubeconfig
    #cp /grdata/kubernetes/admin.kubeconfig /etc/goodrain/kubernetes/

    [ -f "/etc/goodrain/kubernetes/kubeconfig" ] && (
        old_md5=$(md5sum /etc/goodrain/kubernetes/kubeconfig | awk '{print $1}')
        new_md5=$(md5sum /grdata/services/k8s/admin.kubeconfig | awk '{print $1}')
        if [ $old_md5 != $new_md5 ];then
            rm -rf /etc/goodrain/kubernetes/kubeconfig
            rm -rf /etc/goodrain/kubernetes/admin.kubeconfig
            cp /grdata/services/k8s/admin.kubeconfig /etc/goodrain/kubernetes/kubeconfig
            cp /grdata/services/k8s/admin.kubeconfig /etc/goodrain/kubernetes/admin.kubeconfig
        fi
    )
    #rm /etc/goodrain/kubernetes/kubeconfig
    #cp /grdata/kubernetes/admin.kubeconfig /etc/goodrain/kubernetes/kubeconfig
    
    KUBE_API=$(cat /etc/goodrain/envs/ip.sh | awk -F '=' '{print $2}')

    log.stdout '{
            "global":{
                "KUBE_API":"'$KUBE_API',"
            },
            "status":[ 
            { 
                "name":"install_k8s", 
                "condition_type":"INSTALL_K8S", 
                "condition_status":"True"
            } 
            ], 
            "exec_status":"Success",
            "type":"install"
            }'
}

case $1 in
    * )
        prepare
        run
        ;;
esac