#!/bin/bash 

# 需要传入的参数
# TENANT_ID   租户ID
# SERVICE_ID  服务ID
# GITURL      git仓库地址

# variables
TENANT_ID=$1
SERVICE_ID=$2
GITURL=$3
WORK_DIR=$4

if [[ ! -n $TENANT_ID || ! -n $SERVICE_ID || ! -n $GITURL ]];then

  echo "$0 must be set TENANT_ID,SERVICE_ID and Giturl"
  exit 1
fi

LANGUAGE=""
RUNTIMES=""
DEPDS=""
PROCFILE=""

# bin
JQBIN="$WORK_DIR/bin/jq"
GITBIN="/usr/bin/git clone -q"

# path
SOURCE_DIR="/cache/build/${TENANT_ID}/source/${SERVICE_ID}"
LIBDIR="$WORK_DIR/lib"

# 如果之前源码存在删除之
[ -d $SOURCE_DIR ] && rm -rf $SOURCE_DIR

# 源码不存在创建之
[ ! -d  $SOURCE_DIR ] && mkdir -p $SOURCE_DIR

set -e
# 克隆代码
CLONE_TIMEOUT=180
timeout -k 9 $CLONE_TIMEOUT $GITBIN  $GITURL $SOURCE_DIR > /dev/null 2>&1
if [ $? -eq 124 ];then
    echo "timeout in $CLONE_TIMEOUT: $GITURL"
    exit 1
fi
set +e 

# import functions

chmod +x $LIBDIR/*

source $LIBDIR/common.sh
source $LIBDIR/detect_lang
source $LIBDIR/detect_library
source $LIBDIR/detect_procfile
source $LIBDIR/detect_runtimes

#=========== main ==========
LANGUAGE=`detect_lang`

RUNTIMES=`detect_runtimes $LANGUAGE`
DEPDS=`detect_library $LANGUAGE`
PROCFILE=`detect_procfile $LANGUAGE`

result="{\"language\":\"${LANGUAGE:=false}\",\"runtimes\":\"${RUNTIMES:=false}\",\
       \"dependencies\":\"${DEPDS:=false}\",\"procfile\":\"${PROCFILE:=false}\"}"

echo $result
