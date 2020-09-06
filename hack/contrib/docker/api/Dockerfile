FROM goodrainapps/alpine:3.4
LABEL author=zengqg@goodrain.com

ADD rainbond-api /run/rainbond-api
ADD entrypoint.sh /run/entrypoint.sh
WORKDIR /run
ENV RELEASE_DESC=__RELEASE_DESC__
VOLUME [ "/etc/goodrain" ]
ENTRYPOINT ["/run/entrypoint.sh"]