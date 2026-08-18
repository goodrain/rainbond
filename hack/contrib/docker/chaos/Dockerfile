# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS compile
ARG TARGETARCH
ARG RELEASE_DESC
ARG GOPROXY

ENV CGO_ENABLED=1
ENV GOARCH=${TARGETARCH}
ENV GOPROXY=${GOPROXY}
ENV GOOS=linux

COPY . /go/src/github.com/goodrain/rainbond
COPY --from=dameng go/ /tmp/dameng/
WORKDIR /go/src/github.com/goodrain/rainbond
RUN apk --no-cache add binutils-gold gcc g++ git musl-dev sqlite-dev unzip zeromq-dev libsodium-dev && \
    sh scripts/prepare-dameng-go-driver.sh /tmp/dameng && \
    go mod edit -require=dm@v0.0.0 -replace=dm=/tmp/dameng/dm && \
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE" go build -mod=mod -tags "dm sqlite_omit_load_extension netgo" -ldflags "-w -s -X github.com/goodrain/rainbond/cmd.version=$RELEASE_DESC" -o /run/rainbond-chaos ./cmd/builder

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
COPY --from=compile /run/rainbond-chaos /run/rainbond-chaos

WORKDIR /run

ENV RELEASE_DESC=${RELEASE_DESC}

ENTRYPOINT ["/run/entrypoint.sh"]
