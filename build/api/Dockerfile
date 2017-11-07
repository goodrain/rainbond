FROM goodrainapps/alpine:3.4
MAINTAINER zengqg@goodrain.com

ADD rainbond_api /run/rainbond_api
ADD entrypoint.sh /entrypoint.sh
ADD html /run/html
WORKDIR /run
ENV CUR_NET=midonet
ENV EX_DOMAIN=
VOLUME [ "/etc/goodrain" ]
CMD ["/entrypoint.sh"]