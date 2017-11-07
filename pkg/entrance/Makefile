GO_LDFLAGS=-ldflags " -w"
build:
	go build -o entrance ./cmd
release-build:
	docker run -v `pwd`:/go/src/acp_entrance --rm -w /go/src/acp_entrance \
	golang:1.7.3 go build ${GO_LDFLAGS}  -o entrance ./cmd
run:build
	./entrance \
	--plugin-opts user=admin \
	--plugin-opts password=gr123465! \
	--plugin-opts urls=https://test.goodrain.com:9070 \
	--run-mode sync \
	--log-level debug
clean:
	rm -f ./entrance

clean_data:
	curl -XDELETE http://127.0.0.1:2379/v2/keys/entrance?recursive=true

image:release-build
	@docker build -t hub.goodrain.com/dc-deploy/acp_entrance:3.4 .
	@docker push hub.goodrain.com/dc-deploy/acp_entrance:3.4
container:
	@docker run -d --net=host hub.goodrain.com/dc-deploy/acp_entrance:3.4 /entrance \
	--plugin-opts user=admin \
	--plugin-opts password=gr123465! \
	--run-mode sync \
	--plugin-opts urls=https://test.goodrain.com:9070 \
	--token 5ca196801173be06c7e6ce41d5f7b3b8071e680a \
	--api-addr :6200
