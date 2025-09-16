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
RUN apk --no-cache add binutils-gold gcc musl-dev sqlite-dev && go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" -o /run/rainbond-api ./cmd/api;

FROM ubuntu:24.04 AS compress
COPY --from=compile /run/rainbond-api /run/rainbond-api

RUN apt-get update && apt -y install upx && \
    upx --best --lzma /run/rainbond-api
RUN mkdir -p /logs && touch /logs/request.log

FROM rainbond/alpine:3
ARG RELEASE_DESC

ENV RELEASE_DESC=${RELEASE_DESC}
RUN apk add --no-cache libzmq
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/api/entrypoint.sh /run/entrypoint.sh
COPY --from=compress /run/rainbond-api /run/rainbond-api
COPY --from=compress /logs/request.log /logs/request.log

WORKDIR /run

ENTRYPOINT ["/run/entrypoint.sh"]
