FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o block-mechanica main.go

FROM gcr.io/distroless/static

WORKDIR /

COPY --from=builder /app/block-mechanica /block-mechanica

EXPOSE 8080

ENTRYPOINT ["/block-mechanica"]
