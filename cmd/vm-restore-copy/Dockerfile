FROM golang:1.25.0 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags='-s -w' -o /out/vm-restore-copy ./cmd/vm-restore-copy
FROM scratch
COPY --from=build /out/vm-restore-copy /vm-restore-copy
ENTRYPOINT ["/vm-restore-copy"]
