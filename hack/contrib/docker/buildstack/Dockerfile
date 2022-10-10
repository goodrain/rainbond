ARG GO_VERSION
FROM golang:${GO_VERSION}

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk --no-cache add gcc libc-dev tzdata ca-certificates libc6-compat libgcc libstdc++ g++
    
RUN apk add --no-cache binutils-gold

WORKDIR /go

    


