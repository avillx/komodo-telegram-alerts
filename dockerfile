FROM golang:1.25.5-alpine AS builder

WORKDIR /build

RUN go mod init alerter

COPY ./main.go ./main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/alerter main.go

FROM alpine:3.14

WORKDIR /app

ENV USER=runner

RUN adduser -D $USER
USER $USER

COPY --from=builder /build/alerter .
COPY ./LICENSE .

ENTRYPOINT ["./alerter"]
