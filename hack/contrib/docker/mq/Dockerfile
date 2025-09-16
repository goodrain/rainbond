FROM rainbond/golang:1.23-alpine AS compile
ARG TARGETARCH
ARG RELEASE_DESC
ARG GOPROXY

ENV CGO_ENABLED=1
ENV GOARCH=${TARGETARCH}
ENV GOPROXY=${GOPROXY}
ENV GOOS=linux

COPY . /go/src/github.com/goodrain/rainbond
WORKDIR /go/src/github.com/goodrain/rainbond
RUN apk --no-cache add binutils-gold gcc g++ musl-dev sqlite-dev zeromq-dev libsodium-dev && \
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE" CGO_LDFLAGS="-static -lstdc++ -lzmq -lsodium" go build -tags "sqlite_omit_load_extension netgo" -ldflags "-w -s -extldflags '-static -lstdc++ -lzmq -lsodium' -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" -o /run/rainbond-mq ./cmd/mq;

FROM ubuntu:24.04 AS compress
COPY --from=compile /run/rainbond-mq /run/rainbond-mq

RUN apt-get update && apt -y install upx && \
    upx --best --lzma /run/rainbond-mq

FROM rainbond/alpine:3
ARG RELEASE_DESC

ENV RELEASE_DESC=${RELEASE_DESC}
RUN apk add --no-cache libzmq
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/mq/entrypoint.sh /run/entrypoint.sh
COPY --from=compress /run/rainbond-mq /run/rainbond-mq

WORKDIR /run

ENTRYPOINT ["/run/entrypoint.sh"]