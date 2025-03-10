FROM golang:1.23-alpine
ARG TARGETARCH

COPY libzmq /opt/libzmq
ENV PKG_CONFIG_PATH=/opt/libzmq/lib/pkgconfig/

RUN if [ "$TARGETARCH" = "amd64" ]; then \
      apk --no-cache add gcc libc-dev tzdata ca-certificates libc6-compat libgcc libstdc++ g++ binutils-gold pkgconfig gcc musl-dev; \
      cp /opt/libzmq/include/* /usr/include/ && cp -r /opt/libzmq/share/* /usr/share/ && cp -r /opt/libzmq/lib/* /usr/lib/; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
      apk --no-cache add gcc libc-dev tzdata ca-certificates libc6-compat libgcc libstdc++ g++ zeromq-dev gcc libc-dev; \
    fi

WORKDIR /go