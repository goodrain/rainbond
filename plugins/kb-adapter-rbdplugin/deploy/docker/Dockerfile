FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kb-adapter main.go

FROM gcr.io/distroless/static

WORKDIR /

COPY --from=builder /app/kb-adapter /kb-adapter

EXPOSE 8080

ENTRYPOINT ["/kb-adapter"]
