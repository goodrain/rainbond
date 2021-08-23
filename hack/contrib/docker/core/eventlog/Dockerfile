FROM goodrainapps/alpine:3.4
ARG RELEASE_DESC
RUN apk --no-cache add libstdc++ ca-certificates openssl openssl-dev

COPY build/libzmq/lib/libzmq.so.3 /usr/lib

COPY rainbond-eventlog /run/rainbond-eventlog
ADD entrypoint.sh /run/entrypoint.sh

EXPOSE 6366
EXPOSE 6365
EXPOSE 6364
EXPOSE 6363
EXPOSE 6362
VOLUME [ "/etc/goodrain" ]
ENV RELEASE_DESC=${RELEASE_DESC}
ENTRYPOINT  ["/run/entrypoint.sh"]
