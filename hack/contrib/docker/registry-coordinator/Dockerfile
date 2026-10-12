FROM golang:1.25-alpine AS compile
ARG TARGETARCH
ENV CGO_ENABLED=0
ENV GOOS=linux
WORKDIR /source
COPY . .
RUN GOARCH=${TARGETARCH:-amd64} go build -mod=vendor -trimpath -ldflags="-s -w" -o /registry-coordinator ./cmd/registry-coordinator

FROM scratch
COPY --from=compile /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=compile /registry-coordinator /registry-coordinator
EXPOSE 5001
ENTRYPOINT ["/registry-coordinator"]
