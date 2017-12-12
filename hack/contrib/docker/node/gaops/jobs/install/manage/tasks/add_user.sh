#!/bin/bash

function add_user() {
    grep rain /etc/group >/dev/null 2>&1 || groupadd -g 200 rain
    id rain >/dev/null 2>&1 || (
        useradd -m -s /bin/bash -u 200 -g 200 rain
        echo "rain ALL = (root) NOPASSWD:ALL" > /etc/sudoers.d/rain
        chmod 0440 /etc/sudoers.d/rain
    )
    echo "add_user ok"
}

case $1 in
    *)
    add_user
    ;;
esac