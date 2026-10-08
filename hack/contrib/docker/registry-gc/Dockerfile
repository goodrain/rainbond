# Supply the platform-specific native Registry digest observed on this cluster.
# No default tag: a different native version must be an explicit build choice.
ARG REGISTRY_IMAGE
FROM ${REGISTRY_IMAGE} AS native

FROM golang:1.25-alpine AS compile
ARG TARGETARCH
ARG GOPROXY=https://proxy.golang.org,direct
ARG REGISTRY_IMAGE
ENV CGO_ENABLED=0 GOOS=linux
WORKDIR /source
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN digest="${REGISTRY_IMAGE##*@sha256:}" && \
    test "$digest" != "$REGISTRY_IMAGE" && test "${#digest}" = 64 && \
    case "$digest" in *[!0-9a-f]*) exit 1;; esac && \
    GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /registry-gc ./cmd/registry-gc && \
    GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /registry-coordinator ./cmd/registry-coordinator

FROM native
COPY --from=compile /registry-gc /registry-gc
COPY --from=compile /registry-coordinator /registry-coordinator
ENTRYPOINT ["/registry-gc"]
CMD []
