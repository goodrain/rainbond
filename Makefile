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
else
  PUSH = push
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
ifeq ($(WHAT),mq)
	${BIN_PATH}/${BASE_NAME}-mq --log-level=debug
else ifeq ($(WHAT),worker)
	test/run/run_worker.sh ${BIN_PATH}/${BASE_NAME}-worker
else ifeq ($(WHAT),builder)
    ${BIN_PATH}/${BASE_NAME}-chaos \
	--log-level=debug  \
    --mysql="root:@tcp(127.0.0.1:3306)/region"
else
	test/run/run_${WHAT}.sh ${BIN_PATH}/${BASE_NAME}-$(WHAT)
endif	

doc:  
	@cd cmd/api && swagger generate spec -o ../../hack/contrib/docker/api/html/swagger.json
check:
	./check.sh
mock:
	@cd db && ./mock.sh && cd dao && ./mock.sh
help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

