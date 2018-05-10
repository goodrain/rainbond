GO_LDFLAGS=-ldflags " -w"
IMAGE_NAME=hub.goodrain.com/dc-deploy/acp_event-log
IMAGE_TAG=3.2

install:
	@go build ${GO_LDFLAGS}

dev:install
	./event_log --log.level=debug --discover.etcd.addr=http://127.0.0.1:2379 \
	 --db.url="root:admin@tcp(127.0.0.1:3306)/event" \
	 --dockerlog.mode=stream \
	 --message.dockerlog.handle.core.number=2 \
	 --message.garbage.file="/tmp/garbage.log" \
	 --docker.log.homepath="/Users/qingguo/tmp"
dev-cluster:
	@./event_log --log.level=debug --discover.etcd.addr=http://127.0.0.1:2379 \
	 --db.url="root:admin@tcp(127.0.0.1:3306)/event" \
	 --message.garbage.file="/tmp/garbage.log" \
	 --eventlog.bind.port=2000 \
	 --dockerlog.bind.port=2001 \
	 --cluster.bind.port=2002 \
	 --websocket.bind.port=2003\
	 --monitor.subaddress=tcp://127.0.0.1:9441\
	 --docker.log.homepath="/Users/qingguo"

build-alpine:
	@echo "🐳 $@"
	@docker build -t goodraim.me/event-build:v1 build
	@echo "building..."
	@docker run --rm -v `pwd`:/go/src/event_log -w /go/src/event_log goodraim.me/event-build:v1 go build ${GO_LDFLAGS} -o event_log
	@echo "build done."


instance-ali:
	@docker run --name event-log --net host -d -v /var/log/event-log/:/var/log/ ${IMAGE_NAME}:${IMAGE_TAG} \
	  --db.url="root:admin@tcp(172.30.0.9:3306)/event" \
	  --discover.etcd.addr="http://127.0.0.1:4001"

instance-test:build-alpine
	@docker run --name event-log-test --net host -d ${IMAGE_NAME}:${IMAGE_TAG} \
	  --db.url="writer1:CeRYK8UzWD@tcp(10.0.55.72:3306)/region" \
	  --discover.etcd.addr="http://127.0.0.1:4001" \
	  --eventlog.bind.port=2000 \
	  --dockerlog.bind.port=2001 \
	  --cluster.bind.port=2002 \
	  --websocket.bind.port=2003\
	  --monitor.subaddress="tcp://10.0.1.11:9442"

instance-ali-build:build-debian instance-ali
