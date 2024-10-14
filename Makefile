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


# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1"
# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/apis/..." output:crd:artifacts:config=config/crd

# Generate code, controller-gen version: v0.9.2
generate: controller-gen
	chmod +x vendor/k8s.io/code-generator/generate-groups.sh
	./hack/k8s/codegen/update-generated.sh
	$(CONTROLLER_GEN) object:headerFile="hack/k8s/codegen/boilerplate.go.txt" paths="./pkg/apis/..."
	cp -r github.com/goodrain/rainbond/pkg/ ./pkg/
	rm -rf github.com/

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: test
test:
	KUBE_CONFIG=~/.kube/config PROJECT_HOME=${shell pwd} ginkgo -v worker/controllers/helmapp
