GO_LDFLAGS=-ldflags " -w"
VERSION=master
WORK_DIR=/go/src/github.com/goodrain/rainbond
BASE_NAME=rainbond
BASE_DOCKER=./hack/contrib/docker

default: help
all: image ## build linux binaries, build images for docker

clean: 
	@rm -rf ${BIN_PATH}/*

ifeq ($(origin WHAT), undefined)
  WHAT = all
endif
ifeq ($(origin STATIC), undefined)
  STATIC = false
else
  STATIC = true  
endif

ifeq ($(origin GOOS), undefined)
  GOOS = $(shell go env GOOS)
endif

BIN_PATH=./_output/${GOOS}/${VERSION}

ifeq ($(origin PUSH), undefined)
  PUSH = false
endif
.PHONY: build
build:
	@echo "🐳build ${WHAT} ${GOOS}" 
	@GOOS=$(GOOS) ./localbuild.sh $(WHAT)
image:
	@echo "🐳build image ${WHAT}" 	
	@bash ./release.sh ${WHAT} ${PUSH}
binary:
	@echo "🐳build binary ${WHAT} os ${GOOS}"
	@ GOOS=${GOOS} bash ./release.sh binary ${WHAT}
run-c:image
	test/run/run_${WHAT}.sh
run:build
ifeq ($(WHAT),api)
	${BIN_PATH}/${BASE_NAME}-api --log-level=debug \
	--mysql="root:@tcp(127.0.0.1:3306)/region" \
	--kube-config="`PWD`/test/admin.kubeconfig" \
	--api-ssl-enable=true \
	--client-ca-file="`PWD`/test/ssl/ca.pem" \
	--api-ssl-certfile="`PWD`/test/ssl/server.pem" \
	--api-ssl-keyfile="`PWD`/test/ssl/server.key.pem"
	--etcd=http://127.0.0.1:4001,http://127.0.0.1:2379
else ifeq ($(WHAT),mq)
	${BIN_PATH}/${BASE_NAME}-mq --log-level=debug
else ifeq ($(WHAT),worker)
	test/run/run_worker.sh ${BIN_PATH}/${BASE_NAME}-worker
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
else
	test/run/run_${WHAT}.sh ${BIN_PATH}/${BASE_NAME}-$(WHAT)
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

