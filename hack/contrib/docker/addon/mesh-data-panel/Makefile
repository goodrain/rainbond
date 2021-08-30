VERSION=5.1
image:
	docker build -t rainbond/plugin_mesh:${VERSION} .
test:
	docker run -e XDS_HOST_IP=192.168.1.112 -e TENANT_ID=5dbea040f5cb437c82e3bf02943fb02e -e PLUGIN_ID=c5618104b2aa4c508390e4f18f316500 -e SERVICE_NAME=gr439125 --rm -it rainbond/plugin_mesh:5.1	