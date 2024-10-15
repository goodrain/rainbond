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
      -o /run/rainbond-init-probe ./cmd/init-probe; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
      go build -ldflags "-w -s -linkmode external -extldflags '-static' -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" \
      -o /run/rainbond-init-probe ./cmd/init-probe; \
    fi

FROM ubuntu:24.04 as compress
COPY --from=compile /run/rainbond-init-probe /run/rainbond-init-probe

RUN apt-get update && apt -y install upx && \
    upx --best --lzma /run/rainbond-init-probe

FROM rainbond/alpine:3
ARG RELEASE_DESC

ENV RELEASE_DESC=${RELEASE_DESC}
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/init-probe/entrypoint.sh /run/entrypoint.sh
COPY --from=compress /run/rainbond-init-probe /run/rainbond-init-probe

WORKDIR /run

ENTRYPOINT ["/run/entrypoint.sh"]
CMD ["probe"]

