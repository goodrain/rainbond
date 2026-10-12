FROM golang:1.25-alpine AS compile
ARG ALPINE_MIRROR=default
# Keep the image's Alpine release and package signature checks intact.
RUN case "$ALPINE_MIRROR" in \
      default) ;; \
      aliyun) sed -i -e 's#http://dl-cdn\.alpinelinux\.org/#https://mirrors.aliyun.com/#g' -e 's#https://dl-cdn\.alpinelinux\.org/#https://mirrors.aliyun.com/#g' /etc/apk/repositories ;; \
      *) exit 1 ;; \
    esac && \
    apk --no-cache add binutils-gold gcc g++ git musl-dev sqlite-dev zeromq-dev libsodium-dev

ARG TARGETARCH
ARG RELEASE_DESC
ARG GOPROXY

ENV CGO_ENABLED=1
ENV GOARCH=${TARGETARCH}
ENV GOPROXY=${GOPROXY}
ENV GOOS=linux

COPY . /go/src/github.com/goodrain/rainbond
WORKDIR /go/src/github.com/goodrain/rainbond
RUN CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE" go build -tags "sqlite_omit_load_extension netgo" -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" -o /run/rainbond-chaos ./cmd/builder;

FROM ubuntu:24.04 AS compress
RUN apt-get update && apt -y install upx
COPY --from=compile /run/rainbond-chaos /run/rainbond-chaos

RUN upx --best --lzma /run/rainbond-chaos

FROM rainbond/alpine:3
ARG RELEASE_DESC
ARG RAINBOND_ERROR_REPORTING_DSN=
ARG RAINBOND_ERROR_REPORTING_REGION_DSN=
ARG RAINBOND_ERROR_REPORTING_BACKEND_DSN=

ENV RAINBOND_ERROR_REPORTING_DSN=${RAINBOND_ERROR_REPORTING_DSN}
ENV RAINBOND_ERROR_REPORTING_REGION_DSN=${RAINBOND_ERROR_REPORTING_REGION_DSN}
ENV RAINBOND_ERROR_REPORTING_BACKEND_DSN=${RAINBOND_ERROR_REPORTING_BACKEND_DSN}
RUN apk add --no-cache openssl openssh-client subversion libzmq libsodium
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/chaos/entrypoint.sh /run/entrypoint.sh
COPY --from=compile /go/src/github.com/goodrain/rainbond/hack/contrib/docker/chaos/export-app /src/export-app
COPY --from=compress /run/rainbond-chaos /run/rainbond-chaos

WORKDIR /run

ENV RELEASE_DESC=${RELEASE_DESC}

ENTRYPOINT ["/run/entrypoint.sh"]
