GO_LDFLAGS=-ldflags " -w"
VERSION=3.4
WORK_DIR=/go/src/github.com/goodrain/rainbond
BASE_NAME=rainbond
build-mq:
	go build ${GO_LDFLAGS} -o ./build/mq/${BASE_NAME}_mq ./cmd/mq
build-worker:
	go build ${GO_LDFLAGS} -o ./build/worker/${BASE_NAME}_worker ./cmd/worker	
clean:
	@rm -rf ./build/mq/${BASE_NAME}_mq
	@rm -rf ./build/worker/${BASE_NAME}_worker
build-in-container:
	@docker run -v `pwd`:/go/src/${BASE_NAME}_worker -w /go/src/${BASE_NAME}_worker -it golang:1.7.3 bash
run-mq:build-mq
	./build/mq/${BASE_NAME}_mq --log-level=debug
run-worker:build-worker
	CUR_NET=midonet EX_DOMAIN=test-ali.goodrain.net:10080 ./build/worker/${BASE_NAME}_worker \
	--log-level=debug  \
	--db-type=cockroachdb \
	--mysql="postgresql://root@localhost:26257/region" \
	--kube-config=./admin.kubeconfig
run-builder:build-builder
	./build/builder/${BASE_NAME}_builder
run-eventlog:build-eventlog
	./build/eventlog/${BASE_NAME}_eventlog \
	 --log.level=debug --discover.etcd.addr=http://127.0.0.1:2379 \
	 --db.url="root:admin@tcp(127.0.0.1:3306)/event" \
	 --dockerlog.mode=stream \
	 --message.dockerlog.handle.core.number=2 \
	 --message.garbage.file="/tmp/garbage.log" \
	 --docker.log.homepath="/Users/qingguo/tmp"
    
build-builder:
	go build ${GO_LDFLAGS} -o ./build/builder/${BASE_NAME}_builder ./cmd/builder
build-mqcli:
	go build ${GO_LDFLAGS} -o ./build/mqcli/${BASE_NAME}_mqcli ./cmd/mqcli
build-node:
	go build ${GO_LDFLAGS} -o ./build/node/${BASE_NAME}_node ./cmd/node
build-entrance:
	go build ${GO_LDFLAGS} -o ./build/entrance/${BASE_NAME}_entrance ./cmd/entrance	
build-eventlog:
	go build ${GO_LDFLAGS} -o ./build/eventlog/${BASE_NAME}_eventlog ./cmd/eventlog
build-api:
	go build ${GO_LDFLAGS} -o ./build/api/${BASE_NAME}_api ./cmd/api	

build-image-worker:
	@echo "🐳 $@"
	@docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build  ${GO_LDFLAGS}  -o ./build/worker/${BASE_NAME}_worker ./cmd/worker
	@docker build -t hub.goodrain.com/dc-deploy/${BASE_NAME}_worker:${VERSION} ./build/worker
	@rm -f ./build/worker/${BASE_NAME}_worker
build-image-mq:
	@echo "🐳 $@"
	@docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build  ${GO_LDFLAGS}  -o ./build/mq/${BASE_NAME}_mq ./cmd/mq
	@docker build -t hub.goodrain.com/dc-deploy/${BASE_NAME}_mq:${VERSION} ./build/mq
	@rm -f ./build/mq/${BASE_NAME}_mq
build-image-builder:
	@echo "🐳 $@"
	@docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build  ${GO_LDFLAGS}  -o ./build/builder/${BASE_NAME}_builder ./cmd/builder
	@docker build -t hub.goodrain.com/dc-deploy/${BASE_NAME}_chaos:${VERSION} ./build/builder
	@rm -f ./build/builder/${BASE_NAME}_builder
build-image-node:
	@echo "🐳 $@"
	@docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build  ${GO_LDFLAGS}  -o ./build/node/${BASE_NAME}_node ./cmd/node
build-image-entrance:
	@echo "🐳 $@"
	@docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build  ${GO_LDFLAGS}  -o ./build/entrance/${BASE_NAME}_entrance ./cmd/entrance
	@cp -r ./build/dist ./build/entrance/dist
	@docker build -t hub.goodrain.com/dc-deploy/${BASE_NAME}_entrance:${VERSION} ./build/entrance
	@rm -rf ./build/entrance/dist
	@rm -f ./build/entrance/${BASE_NAME}_entrance
build-image-eventlog:
	@echo "🐳 $@"
	@docker build -t goodraim.me/event-build:v1 ./build/eventlog/build
	@echo "building..."
	@docker run --rm -v `pwd`:${WORK_DIR} -w ${WORK_DIR} goodraim.me/event-build:v1 go build  ${GO_LDFLAGS}  -o ./build/eventlog/${BASE_NAME}_eventlog ./cmd/eventlog
	@echo "build done."
	@docker build -t hub.goodrain.com/dc-deploy/${BASE_NAME}_eventlog:${VERSION} ./build/eventlog
	@rm -f ./build/entrance/${BASE_NAME}_eventlog
build-image-api:
	@echo "🐳 $@"
	@docker run -v `pwd`:${WORK_DIR} -w ${WORK_DIR} -it golang:1.8.3 go build  ${GO_LDFLAGS}  -o ./build/api/${BASE_NAME}_api ./cmd/api
	@docker build -t hub.goodrain.com/dc-deploy/${BASE_NAME}_api:${VERSION} ./build/api
	@rm -f ./build/api/${BASE_NAME}_api	
build-image:build-image-worker build-image-mq build-image-builder build-image-eventlog build-image-entrance build-image-node
push-image:
	docker push hub.goodrain.com/dc-deploy/${BASE_NAME}_eventlog:${VERSION}
	docker push hub.goodrain.com/dc-deploy/${BASE_NAME}_entrance:${VERSION}
	docker push hub.goodrain.com/dc-deploy/${BASE_NAME}_chaos:${VERSION}
	docker push hub.goodrain.com/dc-deploy/${BASE_NAME}_mq:${VERSION}
	docker push hub.goodrain.com/dc-deploy/${BASE_NAME}_worker:${VERSION}



	



