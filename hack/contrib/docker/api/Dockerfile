ARG BASE_IMAGE_VERSION
FROM goodrainapps/alpine:${BASE_IMAGE_VERSION}
ARG RELEASE_DESC
RUN apk --no-cache add libstdc++ ca-certificates openssl openssl-dev

COPY ../build/libzmq/lib/libzmq.so.3 /usr/lib
ADD rainbond-api /run/rainbond-api
ADD helm /usr/local/bin/helm
ADD entrypoint.sh /run/entrypoint.sh
WORKDIR /run
ENV RELEASE_DESC=${RELEASE_DESC}
VOLUME [ "/etc/goodrain" ]
ENTRYPOINT ["/run/entrypoint.sh"]
