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
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE" go build -tags "sqlite_omit_load_extension netgo" -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" -o /run/rainbond-worker ./cmd/worker;

FROM ubuntu:24.04 AS compress
COPY --from=compile /run/rainbond-worker /run/rainbond-worker

RUN apt-get update && apt -y install upx && \
    upx --best --lzma /run/rainbond-worker

FROM rainbond/alpine:3
ARG RELEASE_DESC

ENV RELEASE_DESC=${RELEASE_DESC}
RUN apk add --no-cache libzmq libsodium
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/worker/entrypoint.sh /run/entrypoint.sh
COPY --from=compress /run/rainbond-worker /run/rainbond-worker

WORKDIR /run

ENTRYPOINT ["/run/entrypoint.sh"]