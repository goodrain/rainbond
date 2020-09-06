GO_LDFLAGS=-ldflags " -w"
VERSION=$(shell git symbolic-ref HEAD 2>/dev/null | cut -d"/" -f 3)
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
check:
	./check.sh
mock:
	@cd db && ./mock.sh && cd dao && ./mock.sh
help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
precommit:
	./precheck.sh