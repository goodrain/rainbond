FROM alpine:3

COPY ./libzmq.so.3 /usr/lib
RUN apk add --no-cache tzdata libstdc++  libc6-compat tar sed wget curl bash su-exec netcat-openbsd tini && \
       sed -i -e "s/bin\/ash/bin\/bash/" /etc/passwd && \
       ln -s /lib /lib64 && \
       cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
       echo "Asia/Shanghai" >  /etc/timezone && \
       date && apk del --no-cache tzdata

ENV LANG en_US.utf8