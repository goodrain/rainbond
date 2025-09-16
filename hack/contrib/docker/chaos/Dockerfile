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
RUN apk --no-cache add binutils-gold gcc musl-dev sqlite-dev && \
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE" CGO_LDFLAGS="-static" go build -tags "sqlite_omit_load_extension netgo" -ldflags "-w -s -extldflags '-static' -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" -o /run/rainbond-chaos ./cmd/builder;

FROM ubuntu:24.04 AS compress
COPY --from=compile /run/rainbond-chaos /run/rainbond-chaos

RUN apt-get update && apt -y install upx && \
    upx --best --lzma /run/rainbond-chaos

FROM rainbond/alpine:3
ARG RELEASE_DESC

RUN apk add --no-cache openssl openssh-client subversion libzmq
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/chaos/entrypoint.sh /run/entrypoint.sh
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/chaos/export-app /src/export-app
COPY --from=compress /run/rainbond-chaos /run/rainbond-chaos

WORKDIR /run

ENV RELEASE_DESC=${RELEASE_DESC}

ENTRYPOINT ["/run/entrypoint.sh"]
