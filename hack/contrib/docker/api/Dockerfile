FROM goodrainapps/alpine:3.4
ARG RELEASE_DESC
LABEL author=zengqg@goodrain.com
ADD rainbond-api /run/rainbond-api
ADD entrypoint.sh /run/entrypoint.sh
WORKDIR /run
ENV RELEASE_DESC=${RELEASE_DESC}
VOLUME [ "/etc/goodrain" ]
ENTRYPOINT ["/run/entrypoint.sh"]