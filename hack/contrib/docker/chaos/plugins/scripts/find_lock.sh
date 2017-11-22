#!/usr/bin/env bash
for i in `etcdctl ls /goodrain/locks/instances`
do
    etcdctl get $i | INAME=${i##*/} perl -ne 'printf "%-25s: %15s\n",$ENV{INAME},$1 if /holder_identity":\s+"([^"]+)/'
done