FROM golang:1.17-alpine

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && apk --no-cache add pkgconfig gcc musl-dev

COPY ./libzmq /opt/libzmq

ENV PKG_CONFIG_PATH /opt/libzmq/lib/pkgconfig/

RUN cp /opt/libzmq/include/* /usr/include/ && cp -r /opt/libzmq/share/* /usr/share/ \
    && cp -r /opt/libzmq/lib/* /usr/lib/

WORKDIR /go

    


