FROM rainbond/golang:1.19-alpine as compile
ARG TARGETARCH
ARG RELEASE_DESC
ARG GOPROXY

ENV CGO_ENABLED=1
ENV GOARCH=${TARGETARCH}
ENV GOPROXY=${GOPROXY}
ENV GOOS=linux

COPY . /go/src/github.com/goodrain/rainbond
WORKDIR /go/src/github.com/goodrain/rainbond

RUN if [ "$TARGETARCH" = "amd64" ]; then \
      go build -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" \
      -o /run/rainbond-api ./cmd/api; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
      go build -ldflags "-w -s -linkmode external -extldflags '-static' -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" \
      -o /run/rainbond-api ./cmd/api; \
    fi

FROM ubuntu:24.04 as compress
COPY --from=compile /run/rainbond-api /run/rainbond-api

RUN apt-get update && apt -y install upx && \
    upx --best --lzma /run/rainbond-api
RUN mkdir -p /logs && touch /logs/request.log

FROM rainbond/alpine:3
ARG RELEASE_DESC

ENV RELEASE_DESC=${RELEASE_DESC}
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/api/entrypoint.sh /run/entrypoint.sh
COPY --from=compress /run/rainbond-api /run/rainbond-api
COPY --from=compress /logs/request.log /logs/request.log

WORKDIR /run

ENTRYPOINT ["/run/entrypoint.sh"]
