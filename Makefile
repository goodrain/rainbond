GO_LDFLAGS=-ldflags " -w"
VERSION=3.7
WORK_DIR=/go/src/github.com/goodrain/rainbond
BASE_NAME=rainbond
BASE_DOCKER=./hack/contrib/docker
BIN_PATH=./_output/${VERSION}

default: help
all: build images ## build linux binaries, build images for docker

clean: 
	@rm -rf ${BIN_PATH}/*

build_items="api builder entrance grctl monitor mq node webcli worker eventlog"

ifeq ($(origin WHAT), undefined)
  WHAT = all
endif
.PHONY: build
build:
	@echo "🐳build ${WHAT}" 
	@./release.sh localbuild $(WHAT)
image:
	@echo "🐳build image ${WHAT}" 	
	@bash ./release.sh ${WHAT}
deb:
	@bash ./release.sh deb
rpm: 
	@bash ./release.sh rpm
pkg:
	@bash ./release.sh build

run:build
ifeq ($(WHAT),api)
	${BIN_PATH}/${BASE_NAME}-api --log-level=debug \
	--mysql="root:admin@tcp(127.0.0.1:3306)/region" \
	--kube-config="`PWD`/test/admin.kubeconfig" \
	--api-ssl-enable=true \
	--client-ca-file="`PWD`/test/ssl/ca.pem" \
	--api-ssl-certfile="`PWD`/test/ssl/server.pem" \
	--api-ssl-keyfile="`PWD`/test/ssl/server.key.pem"
	--etcd=http://127.0.0.1:4001,http://127.0.0.1:2379
else ifeq ($(WHAT),mq)
	${BIN_PATH}/${BASE_NAME}-mq --log-level=debug
else ifeq ($(WHAT),worker)
	CUR_NET=midonet EX_DOMAIN=test-ali.goodrain.net:10080 ${BIN_PATH}/${BASE_NAME}-worker \
	--log-level=debug  \
	--mysql="root:@tcp(127.0.0.1:3306)/region" \
	--kube-config=./test/admin.kubeconfig
else ifeq ($(WHAT),builder)
    ${BIN_PATH}/${BASE_NAME}-chaos \
	--log-level=debug  \
    --mysql="root:@tcp(127.0.0.1:3306)/region"
else ifeq ($(WHAT),eventlog)
	${BIN_PATH}/${BASE_NAME}-eventlog \
	 --log.level=debug --discover.etcd.addr=http://127.0.0.1:2379 \
	 --db.url="root:@tcp(127.0.0.1:3306)/event" \
	 --dockerlog.mode=stream \
	 --message.dockerlog.handle.core.number=2 \
	 --message.garbage.file="/tmp/garbage.log" \
	 --docker.log.homepath="/Users/qingguo/tmp"
else ifeq ($(WHAT),node)
	${BIN_PATH}/${BASE_NAME}-node \
	 --run-mode=master --kube-conf=`pwd`/test/admin.kubeconfig \
	 --nodeid-file=`pwd`/test/host_id.conf \
	 --static-task-path=`pwd`/test/tasks \
	 --statsd.mapping-config=`pwd`/test/mapper.yml \
	 --log-level=debug	 
endif	

doc:  
	@cd cmd/api && swagger generate spec -o ../../hack/contrib/docker/api/html/swagger.json

cert-ca:
	@_output/3.7/rainbond-certutil create --is-ca --ca-name=./test/ssl/ca.pem --ca-key-name=./test/ssl/ca.key.pem --domains region.goodrain.me
cert-server:
	@_output/3.7/rainbond-certutil create --ca-name=./test/ssl/ca.pem --ca-key-name=./test/ssl/ca.key.pem --crt-name=./test/ssl/server.pem --crt-key-name=./test/ssl/server.key.pem --domains region.goodrain.me
cert-client:
	@_output/3.7/rainbond-certutil create --ca-name=./test/ssl/ca.pem --ca-key-name=./test/ssl/ca.key.pem --crt-name=./test/ssl/client.pem --crt-key-name=./test/ssl/client.key.pem --domains region.goodrain.me
help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo "\033[36m 🤔 single plugin,how to work?   \033[0m"
	@echo "\033[01;34mmake build-<plugin>\033[0m Just like: make build-mq"
	@echo "\033[01;34mmake build-image-<plugin>\033[0m Just like: make build-image-mq"
	@echo "\033[01;34mmake run-<plugin>\033[0m Just like: make run-mq"
