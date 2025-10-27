# 默认目标
.DEFAULT_GOAL := image

# 测试目录，默认递归所有目录
TESTDIR ?= ./...

# 镜像标签，默认 latest
TAG ?= latest

# 构建 Docker 镜像
.PHONY: image
image:
	deploy/docker/build.sh $(TAG)

# 构建 Go 可执行文件
.PHONY: build
build:
	go mod download
	go build -o bin/kb-adapter main.go

# 运行测试
.PHONY: test
test:
	go test $(TESTDIR)

# lint
.PHONY: lint
lint:
	golangci-lint run

# fmt
.PHONY: fmt
fmt:
	golangci-lint fmt

# delpoy
.PHONY: deploy
deploy:
	kubectl apply -f deploy/k8s/deploy.yaml