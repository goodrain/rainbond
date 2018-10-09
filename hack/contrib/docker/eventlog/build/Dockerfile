FROM golang:1.11-alpine

RUN apk --no-cache add pkgconfig gcc musl-dev

COPY ./libzmq /opt/libzmq

ENV PKG_CONFIG_PATH /opt/libzmq/lib/pkgconfig/

RUN cp /opt/libzmq/include/* /usr/include/ && cp -r /opt/libzmq/share/* /usr/share/ \
    && cp -r /opt/libzmq/lib/* /usr/lib/

WORKDIR /go

    


