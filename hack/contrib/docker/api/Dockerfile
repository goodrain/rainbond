FROM goodrainapps/alpine:3.4
MAINTAINER zengqg@goodrain.com

ADD rainbond-api /run/rainbond-api
ADD entrypoint.sh /run/entrypoint.sh
ADD html /run/html
WORKDIR /run
ENV CUR_NET=midonet
ENV EX_DOMAIN=
ENV RELEASE_DESC=__RELEASE_DESC__
VOLUME [ "/etc/goodrain" ]
ENTRYPOINT ["/run/entrypoint.sh"]